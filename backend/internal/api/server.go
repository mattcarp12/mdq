package api

import (
	"net/http"

	"github.com/mattcarp12/mdq/internal/queue"
	"github.com/mattcarp12/mdq/internal/repository"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Server holds all the dependencies required by our HTTP handlers.
type Server struct {
	JobRepo        repository.JobRepository
	QueueProducer  queue.Producer
	UserRepo       repository.UserRepository
	jwtSecret      []byte
	AllowedOrigins []string
}

// NewServer is the constructor for our API server.
func NewServer(repo repository.JobRepository, userRepo repository.UserRepository, queueProducer queue.Producer, jwtSecret []byte, allowedOrigins []string) *Server {
	return &Server{
		JobRepo:        repo,
		QueueProducer:  queueProducer,
		UserRepo:       userRepo,
		jwtSecret:      jwtSecret,
		AllowedOrigins: allowedOrigins,
	}
}

// RegisterRoutes sets up the routing for the server.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Protected routes require authentication middleware
	mux.HandleFunc("POST /api/v1/jobs", s.AuthMiddleware(s.handleCreateJob))
	mux.HandleFunc("GET /api/v1/jobs", s.AuthMiddleware(s.handleListJobs))
	mux.HandleFunc("GET /api/v1/jobs/", s.AuthMiddleware(s.handleGetJob)) // Note: This will match /api/v1/jobs/{id}

	// Public routes
	mux.HandleFunc("POST /api/v1/login", s.handleLogin)
	mux.HandleFunc("GET /healthz", s.HealthCheckHandler)
}

// SetupHandler wraps the multiplexer with global middleware
func (s *Server) SetupHandler() http.Handler {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	// /metrics is registered directly on the mux, outside the logging middleware,
	// so Prometheus scrapes don't flood your request logs.
	mux.Handle("GET /metrics", promhttp.Handler())

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   s.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value for preflight request caching
	})

	// Wrap the mux with OTel HTTP instrumentation
	// This automatically creates a Span for every incoming HTTP request!
	tracedHandler := otelhttp.NewHandler(mux, "http.server")

	// Chain the middleware: CORS -> Logger -> Tracer -> Mux
	return s.LoggingMiddleware(c.Handler(tracedHandler))
}

func (s *Server) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}
