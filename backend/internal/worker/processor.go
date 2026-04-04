package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mattcarp12/mdq/internal/repository"
	"github.com/redis/go-redis/v9"
)

type Processor struct {
	redisClient *redis.Client
	jobRepo     repository.JobRepository
	handlers    map[string]TaskHandler
	streamName  string
	groupName   string
	consumerID  string
}

func NewProcessor(rc *redis.Client, repo repository.JobRepository, stream, group, consumer string) *Processor {
	return &Processor{
		redisClient: rc,
		jobRepo:     repo,
		handlers:    GetHandlers(),
		streamName:  stream,
		groupName:   group,
		consumerID:  consumer,
	}
}

func (p *Processor) Start(ctx context.Context) {
	// 1. Ensure the Consumer Group exists.
	// If it already exists, this returns a specific error we can safely ignore.
	err := p.redisClient.XGroupCreateMkStream(ctx, p.streamName, p.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		slog.Error("failed to create consumer group", slog.String("error", err.Error()))
		return
	}

	slog.Info("Worker started processing...", slog.String("consumer_id", p.consumerID))

	// 2. The Infinite Polling Loop
	for {
		select {
		case <-ctx.Done(): // Triggered during graceful shutdown
			slog.Info("Processor context cancelled, stopping loop...")
			return
		default:
			// Block for up to 2 seconds waiting for new messages
			streams, err := p.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    p.groupName,
				Consumer: p.consumerID,
				Streams:  []string{p.streamName, ">"}, // ">" means "give me messages never delivered to other consumers"
				Count:    1,
				Block:    2 * time.Second,
			}).Result()

			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue // Timeout hit, no new messages, loop again
				}
				if errors.Is(err, context.Canceled) {
					return // Shutdown requested during the block
				}
				slog.Error("failed to read from redis", slog.String("error", err.Error()))
				time.Sleep(1 * time.Second) // Backoff on network failure
				continue
			}

			// 3. Process the messages
			for _, stream := range streams {
				for _, msg := range stream.Messages {
					p.processMessage(ctx, msg)
				}
			}
		}
	}
}

func (p *Processor) processMessage(ctx context.Context, msg redis.XMessage) {
	jobID, ok := msg.Values["job_id"].(string)
	if !ok {
		slog.Error("invalid message format in redis", slog.String("msg_id", msg.ID))
		p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID) // ACK it so we don't see this garbage again
		return
	}

	// Fetch from DB
	job, err := p.jobRepo.GetJobByID(ctx, jobID)
	if err != nil {
		slog.Error("failed to fetch job from db", slog.String("job_id", jobID), slog.String("error", err.Error()))
		return
	}

	// Update UI: Task is now running
	p.jobRepo.UpdateJobState(ctx, jobID, "RUNNING", nil, nil)

	// Look up our chaos handler
	handler, exists := p.handlers[job.Type]
	if !exists {
		errorMsg := fmt.Sprintf("no handler registered for job type: %s", job.Type)
		slog.Error(errorMsg)
		p.jobRepo.UpdateJobState(ctx, jobID, "FAILED", nil, &errorMsg)
		p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID)
		return
	}

	// Execute the actual work!
	err = handler(ctx, job.Payload)

	if err != nil {
		slog.Warn("job execution failed", slog.String("job_id", jobID), slog.String("error", err.Error()))

		// Increment retry counter
		p.jobRepo.IncrementJobRetry(ctx, jobID)
		job.RetriesAttempted++

		if job.RetriesAttempted >= job.MaxRetries {
			// Out of retries. Fail the job and ACK the queue so it stops sending it.
			errMsg := err.Error()
			p.jobRepo.UpdateJobState(ctx, jobID, "FAILED", nil, &errMsg)
			p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID)
			slog.Info("job reached max retries and marked FAILED", slog.String("job_id", jobID))
		} else {
			// We DO NOT send XAck here.
			// Because we don't ACK it, the message stays in the "Pending" state in Redis.
			// (Note: To keep this MVP simple, we are just marking it QUEUED in the DB.
			// A full production system would have a separate "claimer" goroutine that sweeps Redis for un-ACK'd messages older than X minutes and re-enqueues them).
			errMsg := err.Error()
			p.jobRepo.UpdateJobState(ctx, jobID, "QUEUED", nil, &errMsg)
		}
	} else {
		// Success!
		resultMsg := "Successfully processed"
		p.jobRepo.UpdateJobState(ctx, jobID, "COMPLETED", &resultMsg, nil)
		p.redisClient.XAck(ctx, p.streamName, p.groupName, msg.ID) // Remove from Redis forever
	}
}
