package api

import (
	"encoding/json"
	"log/slog"
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

	// Extract the securely validated UserID from the context
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, `{"error": "unauthorized context"}`, http.StatusUnauthorized)
		return
	}

	job, err := s.JobRepo.CreateJob(r.Context(), repository.CreateJobParams{
		Type:       req.Type,
		Payload:    req.Payload,
		MaxRetries: req.MaxRetries,
		UserID:     userID,
	})
	if err != nil {
		slog.Error("Failed to create job in database", "error", err.Error(), "userID", userID, "jobType", req.Type)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	err = s.QueueProducer.Enqueue(r.Context(), job.ID, job.Type)
	if err != nil {
		slog.Error("Failed to enqueue job", "jobID", job.ID, "error", err.Error())
		// Note: We don't fail the HTTP request here. The job is safely in Postgres.
		// Our background "sweeper" process will eventually pick it up and retry the enqueue.
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "Job queued successfully",
		"id":  job.ID,
		"status":  job.Status,
	})
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	// Extract the UserID securely placed by the AuthMiddleware
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		slog.Error("UserID missing in context")
		http.Error(w, `{"error": "unauthorized context"}`, http.StatusUnauthorized)
		return
	}

	jobs, err := s.JobRepo.GetJobsByUser(r.Context(), userID)
	if err != nil {
		slog.Error("Failed to retrieve jobs for user", "userID", userID, "error", err.Error())
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Wrap the array in an object for better JSON API design (easier to add pagination metadata later)
	json.NewEncoder(w).Encode(map[string]any{
		"data": jobs,
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	// Extract the UserID securely placed by the AuthMiddleware
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		slog.Error("UserID missing in context")
		http.Error(w, `{"error": "unauthorized context"}`, http.StatusUnauthorized)
		return
	}

	jobID := r.URL.Path[len("/api/v1/jobs/"):]
	if jobID == "" {
		http.Error(w, `{"error": "job ID is required"}`, http.StatusBadRequest)
		return
	}

	job, err := s.JobRepo.GetJobByID(r.Context(), jobID)
	if err != nil {
		slog.Error("Failed to retrieve job by ID", "jobID", jobID, "error", err.Error())
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}