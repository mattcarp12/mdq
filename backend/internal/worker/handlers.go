package worker

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"
)

// TaskHandler defines the signature for all our background jobs.
// It takes the raw JSON payload from the database and returns an error if it fails.
type TaskHandler func(ctx context.Context, payload []byte) error

// GetHandlers returns our routing map. 
// When the worker pulls a job, it looks up the job.Type here to execute the correct function.
func GetHandlers() map[string]TaskHandler {
	return map[string]TaskHandler{
		"video_process":    handleVideoProcess,
		"create_thumbnail": handleCreateThumbnail,
		"send_email":       handleSendEmail,
	}
}

// simulateWork is our internal chaos engineer. 
func simulateWork(base time.Duration, jitter time.Duration, failProb float32) error {
	// 1. Calculate random jitter and sleep
	// rand.Float64() returns a value between 0.0 and 1.0
	jitterAmount := time.Duration(rand.Float64() * float64(jitter))
	sleepTime := base + jitterAmount
	
	time.Sleep(sleepTime)

	// 2. Roll the dice for failure
	if rand.Float32() < failProb {
		return fmt.Errorf("simulated random failure occurred after %v", sleepTime)
	}

	return nil
}

// ---------------------------------------------------------
// Individual Task Implementations
// ---------------------------------------------------------

func handleVideoProcess(ctx context.Context, payload []byte) error {
	slog.Info("Starting video_process task", slog.String("payload", string(payload)))
	
	// Video processing is heavy: 3 seconds base time, up to 2 seconds of jitter, 40% fail rate
	err := simulateWork(3*time.Second, 2*time.Second, 0.40)
	if err != nil {
		return fmt.Errorf("ffmpeg encoding failed: %w", err)
	}

	slog.Info("video_process completed successfully")
	return nil
}

func handleCreateThumbnail(ctx context.Context, payload []byte) error {
	slog.Info("Starting create_thumbnail task", slog.String("payload", string(payload)))
	
	// Thumbnails are fast: 500ms base, 500ms jitter, 10% fail rate
	err := simulateWork(500*time.Millisecond, 500*time.Millisecond, 0.10)
	if err != nil {
		return fmt.Errorf("imagemagick processing failed: %w", err)
	}

	slog.Info("create_thumbnail completed successfully")
	return nil
}

func handleSendEmail(ctx context.Context, payload []byte) error {
	slog.Info("Starting send_email task", slog.String("payload", string(payload)))
	
	// Emails rely on external APIs (like SendGrid): 1s base, 1s jitter, 20% fail rate (simulating network timeout)
	err := simulateWork(1*time.Second, 1*time.Second, 0.20)
	if err != nil {
		return fmt.Errorf("smtp connection timeout: %w", err)
	}

	slog.Info("send_email completed successfully")
	return nil
}