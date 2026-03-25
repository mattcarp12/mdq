package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mattcarp12/mdq/internal/models"
)

// CreateJobParams defines exactly what is required to create a new job.
type CreateJobParams struct {
	Type       string
	Payload    []byte // json.RawMessage is a byte slice under the hood
	MaxRetries int
	UserID     string
}

type JobRepository interface {
	// Notice we pass Params by value (immutable) and return a new Job by value
	CreateJob(ctx context.Context, params CreateJobParams) (models.Job, error)
	UpdateJobStatus(ctx context.Context, id string, status models.JobStatus) error
	GetJobsByUser(ctx context.Context, userID string) ([]models.Job, error)
}

type postgresJobRepo struct {
	db *pgxpool.Pool
}

// NewPostgresJobRepository injects the connection pool into our repository.
func NewPostgresJobRepository(db *pgxpool.Pool) JobRepository {
	return &postgresJobRepo{db: db}
}

func (r *postgresJobRepo) CreateJob(ctx context.Context, params CreateJobParams) (models.Job, error) {
	query := `
		INSERT INTO jobs (type, payload, status, max_retries, user_id)
		VALUES ($1, $2, 'PENDING', $3, $4)
		RETURNING id, type, payload, status, max_retries, retries_attempted, run_at, created_at, updated_at
	`

	var job models.Job

	err := r.db.QueryRow(
		ctx, 
		query, 
		params.Type, 
		params.Payload, 
		params.MaxRetries,
		params.UserID, // NEW: Passing the UserID to Postgres
	).Scan(
		&job.ID,
		&job.Type,
		&job.Payload,
		&job.Status,
		&job.MaxRetries,
		&job.RetriesAttempted,
		&job.RunAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return models.Job{}, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// UpdateJobStatus is used by the workers to transition a job from PENDING -> RUNNING -> COMPLETED.
func (r *postgresJobRepo) UpdateJobStatus(ctx context.Context, id string, status models.JobStatus) error {
	query := `
		UPDATE jobs 
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	commandTag, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("job with id %s not found", id)
	}

	return nil
}

func (r *postgresJobRepo) GetJobsByUser(ctx context.Context, userID string) ([]models.Job, error) {
	query := `
		SELECT id, type, payload, status, result, error_details, max_retries, retries_attempted, run_at, created_at, updated_at
		FROM jobs 
		WHERE user_id = $1 
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	// Initialize with an empty slice rather than nil. 
	// This ensures our API returns `[]` instead of `null` if the user has no jobs.
	jobs := make([]models.Job, 0)

	for rows.Next() {
		var job models.Job
		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Payload,
			&job.Status,
			&job.Result,
			&job.ErrorDetails,
			&job.MaxRetries,
			&job.RetriesAttempted,
			&job.RunAt,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return jobs, nil
}