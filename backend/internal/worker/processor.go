package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mattcarp12/mdq/internal/models"
	"github.com/mattcarp12/mdq/internal/repository"
	"github.com/redis/go-redis/v9"
)

type Processor struct {
	redisClient       *redis.Client
	jobRepo           repository.JobRepository
	handlers          map[string]TaskHandler
	streamName        string
	groupName         string
	consumerID        string
	minIdleTime       time.Duration
	autoclaimInterval time.Duration
}

func NewProcessor(rc *redis.Client, repo repository.JobRepository, stream, group, consumer string) *Processor {
	return &Processor{
		redisClient:       rc,
		jobRepo:           repo,
		handlers:          GetHandlers(),
		streamName:        stream,
		groupName:         group,
		consumerID:        consumer,
		minIdleTime:       5 * time.Minute, // If a job sits un-ACK'd for 5 mins, it's dead
		autoclaimInterval: 1 * time.Minute, // Check for dead jobs every 60 seconds
	}
}

func (p *Processor) Start(ctx context.Context) {
	err := p.redisClient.XGroupCreateMkStream(ctx, p.streamName, p.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		slog.Error("failed to create consumer group", slog.String("error", err.Error()))
		return
	}

	slog.Info("Worker started processing...", slog.String("consumer_id", p.consumerID))

	lastClaim := time.Now()

	// The SOTA Unified Event Loop: Clean, readable, and delegates the heavy lifting
	for {
		select {
		case <-ctx.Done():
			slog.Info("Processor context cancelled, stopping loop...")
			return
		default:
			// 1. Check for abandoned jobs
			if time.Since(lastClaim) > p.autoclaimInterval {
				p.handleAbandonedMessages(ctx)
				lastClaim = time.Now()
			}

			// 2. Poll for new jobs
			streams, err := p.dequeueMessages(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				slog.Error("failed to read from redis", slog.String("error", err.Error()))
				time.Sleep(1 * time.Second) // Backoff on network failure
				continue
			}

			// 3. Process the batch
			for _, stream := range streams {
				for _, msg := range stream.Messages {
					p.processMessage(ctx, msg)
				}
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Infrastructure Delegation Methods
// -----------------------------------------------------------------------------

// handleAbandonedMessages rescues jobs that previous workers died while processing
func (p *Processor) handleAbandonedMessages(ctx context.Context) {
	messages, _, err := p.redisClient.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   p.streamName,
		Group:    p.groupName,
		MinIdle:  p.minIdleTime,
		Consumer: p.consumerID,
		Count:    1,
		Start:    "0-0",
	}).Result()

	if err != nil && !errors.Is(err, redis.Nil) {
		slog.Error("failed to run XAUTOCLAIM", slog.String("error", err.Error()))
		return
	}

	if len(messages) > 0 {
		slog.Warn("rescued abandoned jobs via XAUTOCLAIM", slog.Int("count", len(messages)))
		for _, msg := range messages {
			p.processMessage(ctx, msg)
		}
	}
}

// dequeueMessages blocks and waits for new jobs to arrive in the stream
func (p *Processor) dequeueMessages(ctx context.Context) ([]redis.XStream, error) {
	streams, err := p.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    p.groupName,
		Consumer: p.consumerID,
		Streams:  []string{p.streamName, ">"},
		Count:    1,
		Block:    2 * time.Second,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Timeout hit, no new messages. Not an error.
		}
		return nil, err
	}

	return streams, nil
}

// -----------------------------------------------------------------------------
// Business Logic
// -----------------------------------------------------------------------------

func (p *Processor) processMessage(ctx context.Context, msg redis.XMessage) {
	jobID, ok := msg.Values["job_id"].(string)
	if !ok {
		slog.Error("invalid message format in redis", slog.String("msg_id", msg.ID))
		p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID)
		return
	}

	job, err := p.jobRepo.GetJobByID(ctx, jobID)
	if err != nil {
		slog.Error("failed to fetch job from db", slog.String("job_id", jobID), slog.String("error", err.Error()))
		return
	}

	if err := p.jobRepo.UpdateJobState(ctx, jobID, models.StatusRunning, nil, nil); err != nil {
		slog.Error("failed to update job to RUNNING", slog.String("error", err.Error()))
	}

	handler, exists := p.handlers[job.Type]
	if !exists {
		errorMsg := fmt.Sprintf(`{"error": "no handler registered for job type: %s"}`, job.Type)
		slog.Error("missing handler", slog.String("job_type", job.Type))

		if err := p.jobRepo.UpdateJobState(ctx, jobID, models.StatusFailed, nil, &errorMsg); err != nil {
			slog.Error("failed to update job to FAILED", slog.String("error", err.Error()))
		}
		p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID)
		return
	}

	err = handler(ctx, job.Payload)

	if err != nil {
		slog.Warn("job execution failed", slog.String("job_id", jobID), slog.String("error", err.Error()))

		p.jobRepo.IncrementJobRetry(ctx, jobID)
		job.RetriesAttempted++

		errMsgJSON := fmt.Sprintf(`{"error": "%s"}`, err.Error())

		if job.RetriesAttempted >= job.MaxRetries {
			if err := p.jobRepo.UpdateJobState(ctx, jobID, models.StatusFailed, nil, &errMsgJSON); err != nil {
				slog.Error("failed to update job to FAILED", slog.String("error", err.Error()))
			}
			p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID)
		} else {
			if err := p.jobRepo.UpdateJobState(ctx, jobID, models.StatusRetrying, nil, &errMsgJSON); err != nil {
				slog.Error("failed to update job to RETRYING", slog.String("error", err.Error()))
			}
		}
	} else {
		resultMsg := `{"message": "Successfully processed"}`
		if err := p.jobRepo.UpdateJobState(ctx, jobID, models.StatusCompleted, &resultMsg, nil); err != nil {
			slog.Error("failed to update job to COMPLETED", slog.String("error", err.Error()))
		} else {
			p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID)
		}
	}
}
