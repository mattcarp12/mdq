package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/mattcarp12/mdq/internal/repository"
)

type JobRequest struct {
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	MaxRetries int             `json:"max_retries"`
}

// handleCreateJob is now neatly isolated.
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON payload"}`, http.StatusBadRequest)
		return
	}

	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}

	job, err := s.JobRepo.CreateJob(r.Context(), repository.CreateJobParams{
		Type:       req.Type,
		Payload:    req.Payload,
		MaxRetries: req.MaxRetries,
	})

	if err != nil {
		log.Printf("Failed to create job: %v", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	err = s.QueueProducer.Enqueue(r.Context(), job.ID, job.Type)
	if err != nil {
		log.Printf("Failed to enqueue job %s: %v", job.ID, err)
		// Note: We don't fail the HTTP request here. The job is safely in Postgres.
		// Our background "sweeper" process will eventually pick it up and retry the enqueue.
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "Job queued successfully",
		"job_id":  job.ID,
		"status":  job.Status,
	})
}