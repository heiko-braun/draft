package reviewd

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// Server is the review service HTTP server.
type Server struct {
	db      *sql.DB
	store   *Store
	auth    *AuthMiddleware
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
