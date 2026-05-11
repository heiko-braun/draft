package reviewd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Server is the review service HTTP server.
type Server struct {
	db      *sql.DB
	store   *Store
	auth    *AuthMiddleware
	hub     *Hub
	mux     *http.ServeMux
	handler http.Handler
	logger  *Logger
	config  Config
}

// NewServer creates a new review service server.
func NewServer(db *sql.DB, config Config, logger *Logger) *Server {
	s := &Server{
		db:     db,
		store:  NewStore(db),
		auth:   NewAuthMiddleware(logger),
		hub:    NewHub(logger),
		mux:    http.NewServeMux(),
		logger: logger,
		config: config,
	}
	s.routes()
	s.handler = s.auth.Middleware(s.mux)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.logger.Debug("incoming request",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"remote", r.RemoteAddr,
		"content_length", fmt.Sprintf("%d", r.ContentLength),
	)
	lw := &statusWriter{ResponseWriter: w, status: 200}
	s.handler.ServeHTTP(lw, r)
	s.logger.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", http.StatusText(lw.status),
		"duration_ms", time.Since(start).Truncate(time.Millisecond).String(),
	)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	read := s.auth.RequireRepoAccess(AccessRead)
	write := s.auth.RequireRepoAccess(AccessWrite)

	// Thread endpoints
	s.mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/threads", read(s.handleListThreads))
	s.mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/threads/{threadID}", read(s.handleGetThread))
	s.mux.HandleFunc("PUT /api/v1/repos/{owner}/{repo}/threads/{threadID}", write(s.handlePutThread))
	s.mux.HandleFunc("DELETE /api/v1/repos/{owner}/{repo}/threads/{threadID}", write(s.handleDeleteThread))

	// Comment endpoints
	s.mux.HandleFunc("POST /api/v1/repos/{owner}/{repo}/threads/{threadID}/comments", write(s.handleAddComment))

	// Review endpoints
	s.mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/reviews", read(s.handleListReviews))
	s.mux.HandleFunc("POST /api/v1/repos/{owner}/{repo}/reviews", write(s.handleCreateReview))
	s.mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/reviews/{reviewID}", read(s.handleGetReview))
	s.mux.HandleFunc("PATCH /api/v1/repos/{owner}/{repo}/reviews/{reviewID}", write(s.handlePatchReview))

	// Sync endpoints
	s.mux.HandleFunc("POST /api/v1/repos/{owner}/{repo}/sync", read(s.handleSync))
	s.mux.HandleFunc("POST /api/v1/repos/{owner}/{repo}/publish", write(s.handlePublish))

	// SSE events
	s.mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/events", read(s.hub.HandleSSE(s.store)))
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := s.db.PingContext(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unavailable", "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// statusWriter wraps ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}
