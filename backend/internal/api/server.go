package api

import (
	"net/http"

	"github.com/mattcarp12/mdq/internal/queue"
	"github.com/mattcarp12/mdq/internal/repository"
)

// Server holds all the dependencies required by our HTTP handlers.
type Server struct {
	JobRepo repository.JobRepository
	QueueProducer queue.Producer
}

// NewServer is the constructor for our API server.
func NewServer(repo repository.JobRepository, queueProducer queue.Producer) *Server {
	return &Server{
		JobRepo: repo,
		QueueProducer: queueProducer,
	}
}

// RegisterRoutes sets up the routing for the server.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/jobs", s.handleCreateJob)
}
