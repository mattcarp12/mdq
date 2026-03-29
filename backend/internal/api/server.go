package api

import (
	"net/http"

	"github.com/mattcarp12/mdq/internal/queue"
	"github.com/mattcarp12/mdq/internal/repository"
	"github.com/rs/cors"
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
	mux.HandleFunc("GET /healthz", s.HealthCheckHandler)
}

// SetupHandler wraps the multiplexer with global middleware
func (s *Server) SetupHandler() http.Handler {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	// Configure CORS
	c := cors.New(cors.Options{
		// TODO: In a true SOTA app, you'd load these from an ENV var
		AllowedOrigins: []string{
			"http://localhost:5173",    // Local Vite dev
			"https://*.cloudfront.net", // Production (you can narrow this to your specific URL later)
			"https://*.amoritacleaning.com", // Production (you can narrow this to your specific URL later)
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value for preflight request caching
	})

	// Wrap the entire mux in the LoggingMiddleware
	return s.LoggingMiddleware(c.Handler(mux))
}

func (s *Server) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
