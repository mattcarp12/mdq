package models

import (
	"encoding/json"
	"time"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	StatusPending   JobStatus = "PENDING"
	StatusRunning   JobStatus = "RUNNING"
	StatusCompleted JobStatus = "COMPLETED"
	StatusFailed    JobStatus = "FAILED"
	StatusRetrying  JobStatus = "RETRYING"
)

// Job represents a single unit of work in the system
type Job struct {
	ID               string          `json:"id"` // Assuming UUID is read as string
	Type             string          `json:"type"`
	Payload          json.RawMessage `json:"payload"` // json.RawMessage defers decoding
	Status           JobStatus       `json:"status"`
	Result           json.RawMessage `json:"result,omitempty"`
	ErrorDetails     *string         `json:"error_details,omitempty"`
	MaxRetries       int             `json:"max_retries"`
	RetriesAttempted int             `json:"retries_attempted"`
	RunAt            time.Time       `json:"run_at"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}