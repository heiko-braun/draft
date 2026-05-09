package review

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Server serves the review UI and JSON API.
type Server struct {
	store        ReviewStore
	syncer       ReviewSyncer
	docIndex     *DocIndex
	config       ReviewConfig
	docsRoot     string
	repoRoot     string
	sourceBranch string
	repoName     string
	userEmail    string
	debug        bool
	mux          *http.ServeMux
	logger       *log.Logger
}

// NewServer creates a Server wired to the given store, syncer, and document index.
func NewServer(
	store ReviewStore,
	syncer ReviewSyncer,
	docIndex *DocIndex,
	config ReviewConfig,
	docsRoot, repoRoot, sourceBranch, repoName, userEmail string,
	debug bool,
) *Server {
	s := &Server{
		store:        store,
		syncer:       syncer,
		docIndex:     docIndex,
		config:       config,
		docsRoot:     docsRoot,
		repoRoot:     repoRoot,
		sourceBranch: sourceBranch,
		repoName:     repoName,
		userEmail:    userEmail,
		debug:        debug,
		mux:          http.NewServeMux(),
		logger:       log.New(os.Stderr, "[review] ", log.Ltime),
	}
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.debug {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		s.mux.ServeHTTP(lw, r)
		s.logger.Printf("%s %s %d %s", r.Method, r.URL.Path, lw.status, time.Since(start).Round(time.Millisecond))
	} else {
		s.mux.ServeHTTP(w, r)
	}
}

// loggingResponseWriter captures the status code for logging.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/documents", s.handleDocuments)
	s.mux.HandleFunc("/api/documents/", s.handleDocumentDetail)
	s.mux.HandleFunc("/api/reviews", s.handleReviews)
	s.mux.HandleFunc("/api/threads", s.handleThreads)
	s.mux.HandleFunc("/api/threads/", s.handleThreadAction)
	s.mux.HandleFunc("/api/sync", s.handleSync)
	s.mux.HandleFunc("/api/publish", s.handlePublish)
	s.mux.HandleFunc("/api/status", s.handleStatus)
}

// --- Handlers ---

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, reviewUIHTML)
}

func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var items []DocumentListItem
	for path, doc := range s.docIndex.Documents {
		threads, _ := s.store.ListThreadsByDocument(path)
		openCount := 0
		for _, t := range threads {
			if t.Status == ThreadOpen {
				openCount++
			}
		}
		var modTime int64
		absPath := filepath.Join(s.docsRoot, path)
		if info, err := os.Stat(absPath); err == nil {
			modTime = info.ModTime().Unix()
		}
		items = append(items, DocumentListItem{
			Path:        path,
			Title:       doc.Title,
			Status:      doc.FrontMatter.Status,
			ThreadCount: openCount,
			ModTime:     modTime,
		})
	}

	writeJSONResponse(w, items)
}

func (s *Server) handleDocumentDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docPath := strings.TrimPrefix(r.URL.Path, "/api/documents/")
	if docPath == "" {
		http.Error(w, "document path required", http.StatusBadRequest)
		return
	}

	doc, ok := s.docIndex.Documents[docPath]
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Read raw markdown and render to HTML.
	absPath := filepath.Join(s.docsRoot, docPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		http.Error(w, "failed to read document", http.StatusInternalServerError)
		return
	}

	_, body := ExtractFrontMatter(content)
	htmlContent, err := renderReviewMarkdown(body)
	if err != nil {
		http.Error(w, "failed to render markdown", http.StatusInternalServerError)
		return
	}

	// Compute file hash for anchor staleness detection.
	fileHash := fmt.Sprintf("%x", sha256.Sum256(content))

	threads, _ := s.store.ListThreadsByDocument(docPath)

	detail := DocumentDetail{
		Path:     docPath,
		Title:    doc.Title,
		HTML:     htmlContent,
		FileHash: fileHash,
		Threads:  threads,
		Metadata: doc.FrontMatter,
	}

	writeJSONResponse(w, detail)
}

func (s *Server) handleReviews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		reviews, err := s.store.ListReviews()
		if err != nil {
			httpError(w, "failed to list reviews", err)
			return
		}
		writeJSONResponse(w, reviews)

	case http.MethodPost:
		var req CreateReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		review, err := s.store.CreateReview(req.Title, req.Documents, req.SourceRef)
		if err != nil {
			httpError(w, "failed to create review", err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSONResponse(w, review)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleThreads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		docPath := r.URL.Query().Get("document")
		if docPath == "" {
			http.Error(w, "document query parameter required", http.StatusBadRequest)
			return
		}
		threads, err := s.store.ListThreadsByDocument(docPath)
		if err != nil {
			httpError(w, "failed to list threads", err)
			return
		}
		if threads == nil {
			threads = []Thread{}
		}
		writeJSONResponse(w, threads)

	case http.MethodPost:
		var req CreateThreadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		thread, err := s.store.CreateThread(req.ReviewID, req.Document, req.Anchor)
		if err != nil {
			httpError(w, "failed to create thread", err)
			return
		}
		// Add the initial comment.
		author := req.Author
		if author == "" {
			author = s.userEmail
		}
		if req.Body != "" {
			if _, err := s.store.AddComment(req.Document, thread.ID, author, req.Body); err != nil {
				httpError(w, "thread created but failed to add comment", err)
				return
			}
			// Re-read to include the comment.
			thread, _ = s.store.GetThread(req.Document, thread.ID)
		}
		w.WriteHeader(http.StatusCreated)
		writeJSONResponse(w, thread)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// threadActionPattern matches /api/threads/{id}/action
var threadActionPattern = regexp.MustCompile(`^/api/threads/([^/]+)/(comments|resolve|reopen|delete)$`)

func (s *Server) handleThreadAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m := threadActionPattern.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return
	}

	threadID := m[1]
	action := m[2]

	// We need the document path to locate the thread file.
	docPath := r.URL.Query().Get("document")
	if docPath == "" {
		http.Error(w, "document query parameter required", http.StatusBadRequest)
		return
	}

	switch action {
	case "comments":
		var req AddCommentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		author := req.Author
		if author == "" {
			author = s.userEmail
		}
		comment, err := s.store.AddComment(docPath, threadID, author, req.Body)
		if err != nil {
			httpError(w, "failed to add comment", err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSONResponse(w, comment)

	case "resolve":
		if err := s.store.ResolveThread(docPath, threadID); err != nil {
			httpError(w, "failed to resolve thread", err)
			return
		}
		writeJSONResponse(w, map[string]string{"status": "resolved"})

	case "reopen":
		if err := s.store.ReopenThread(docPath, threadID); err != nil {
			httpError(w, "failed to reopen thread", err)
			return
		}
		writeJSONResponse(w, map[string]string{"status": "open"})

	case "delete":
		if err := s.store.DeleteThread(docPath, threadID); err != nil {
			httpError(w, "failed to delete thread", err)
			return
		}
		writeJSONResponse(w, map[string]string{"status": "deleted"})
	}
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.syncer.SyncAll()
	if err != nil {
		writeJSONResponse(w, SyncResponse{OK: false, Error: err.Error()})
		return
	}

	// Re-index documents after sync.
	newIndex, indexErr := IndexDocuments(s.docsRoot, s.config.DocumentPaths)
	if indexErr == nil {
		s.docIndex = newIndex
	}

	writeJSONResponse(w, SyncResponse{OK: true, Message: "sync complete"})
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.syncer.Publish()
	if err != nil {
		writeJSONResponse(w, PublishResponse{OK: false, Error: err.Error()})
		return
	}

	writeJSONResponse(w, PublishResponse{OK: true, Message: "published successfully"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	openReviews, _ := s.store.ListOpenReviews()
	allThreads, _ := s.store.ListAllThreads()
	pending, _ := s.syncer.HasPendingChanges()

	openThreads := 0
	for _, t := range allThreads {
		if t.Status == ThreadOpen {
			openThreads++
		}
	}

	writeJSONResponse(w, StatusResponse{
		RepoName:       s.repoName,
		Branch:         s.sourceBranch,
		PendingChanges: pending,
		OpenReviews:    len(openReviews),
		OpenThreads:    openThreads,
		TotalThreads:   len(allThreads),
	})
}

// --- Helpers ---

func writeJSONResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func httpError(w http.ResponseWriter, msg string, err error) {
	http.Error(w, fmt.Sprintf("%s: %v", msg, err), http.StatusInternalServerError)
}

// renderReviewMarkdown renders markdown to HTML using goldmark, adding
// data-paragraph-index attributes to <p> tags for anchor positioning.
func renderReviewMarkdown(source []byte) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
