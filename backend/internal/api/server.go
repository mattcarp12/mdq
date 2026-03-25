package api

import (
	"net/http"

	"github.com/mattcarp12/mdq/internal/queue"
	"github.com/mattcarp12/mdq/internal/repository"
)

// Server holds all the dependencies required by our HTTP handlers.
type Server struct {
	JobRepo       repository.JobRepository
	QueueProducer queue.Producer
	UserRepo      repository.UserRepository
	jwtSecret     []byte
}

// NewServer is the constructor for our API server.
func NewServer(repo repository.JobRepository, userRepo repository.UserRepository, queueProducer queue.Producer, jwtSecret []byte) *Server {
	return &Server{
		JobRepo:       repo,
		QueueProducer: queueProducer,
		UserRepo:      userRepo,
		jwtSecret:     jwtSecret,
	}
}

// RegisterRoutes sets up the routing for the server.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Protected routes require authentication middleware
	mux.HandleFunc("POST /api/v1/jobs", s.AuthMiddleware(s.handleCreateJob))
	mux.HandleFunc("GET /api/v1/jobs", s.AuthMiddleware(s.handleListJobs))

	// Public routes
	mux.HandleFunc("POST /api/v1/login", s.handleLogin)
}

// SetupHandler wraps the multiplexer with global middleware
func (s *Server) SetupHandler() http.Handler {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	// Wrap the entire mux in the LoggingMiddleware
	return s.LoggingMiddleware(mux)
}
