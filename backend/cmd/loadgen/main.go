package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	totalJobs       = 100 // How many jobs to create
	concurrentUsers = 10  // How many simultaneous connections
)

// CLI Flags
var (
	serverURL string
	email     string
	password  string
)

func init() {
	flag.StringVar(&serverURL, "server", "http://localhost:8080", "Base URL of the API server")
	flag.StringVar(&email, "email", "test@example.com", "User email for JWT authentication")
	flag.StringVar(&password, "password", "password123", "User password for JWT authentication")
}

// Request/Response Structs
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type CreateJobRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type JobResponse struct {
	ID     string `json:"job_id"`
	Status string `json:"status"`
}

func main() {
	// Parse the CLI flags before doing anything else
	flag.Parse()

	slog.Info("Starting MDQ Load Generator...",
		slog.String("target", serverURL),
		slog.Int("total_jobs", totalJobs),
		slog.Int("concurrency", concurrentUsers),
	)

	// 0. AUTHENTICATE: Get the JWT first
	slog.Info("Authenticating user...", slog.String("email", email))
	token, err := login()
	if err != nil {
		slog.Error("Failed to authenticate. Halting load test.", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("Authentication successful! JWT acquired.")

	var successfulCreations atomic.Int32
	var failedCreations atomic.Int32

	jobIDs := make(chan string, totalJobs)
	var wg sync.WaitGroup

	// 1. THE SWARM: Launch concurrent workers
	startTime := time.Now()

	workChan := make(chan int, totalJobs)
	for i := 0; i < totalJobs; i++ {
		workChan <- i
	}
	close(workChan)

	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range workChan {
				// Pass the JWT to the creation function
				jobID, err := createJob(token)
				if err != nil {
					slog.Error("failed to create job", slog.String("error", err.Error()))
					failedCreations.Add(1)
					continue
				}
				successfulCreations.Add(1)
				jobIDs <- jobID
			}
		}()
	}

	wg.Wait()
	close(jobIDs)
	creationTime := time.Since(startTime)

	slog.Info("Job submission complete",
		slog.Duration("duration", creationTime),
		slog.Int("success", int(successfulCreations.Load())),
		slog.Int("failed", int(failedCreations.Load())),
	)

	// 2. THE E2E VERIFICATION: Poll the API
	slog.Info("Starting E2E verification...")

	var pendingJobs []string
	for id := range jobIDs {
		pendingJobs = append(pendingJobs, id)
	}

	for len(pendingJobs) > 0 {
		var remainingJobs []string

		for _, id := range pendingJobs {
			status, err := checkJobStatus(id, token)
			if err != nil {
				slog.Error("failed to check status", slog.String("id", id), slog.String("error", err.Error()))
				remainingJobs = append(remainingJobs, id) // Retry later
				continue
			}

			if status == "COMPLETED" || status == "FAILED" {
				slog.Info("Job finished", slog.String("id", id), slog.String("final_status", status))
			} else {
				remainingJobs = append(remainingJobs, id)
			}
		}

		pendingJobs = remainingJobs
		if len(pendingJobs) > 0 {
			slog.Info("Waiting for pending jobs...", slog.Int("remaining", len(pendingJobs)))
			time.Sleep(2 * time.Second)
		}
	}

	slog.Info("All jobs processed!", slog.Duration("total_e2e_time", time.Since(startTime)))
}

// ---------------------------------------------------------------------
// HTTP Helpers
// ---------------------------------------------------------------------

func login() (string, error) {
	loginURL := fmt.Sprintf("%s/api/v1/login", serverURL)
	reqBody := LoginRequest{Email: email, Password: password}
	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected login status: %d", resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}

	return loginResp.Token, nil
}

func createJob(token string) (string, error) {
	jobURL := fmt.Sprintf("%s/api/v1/jobs", serverURL)
	reqBody := CreateJobRequest{
		Type:    "video_process",
		Payload: json.RawMessage(`{"video_url": "s3://bucket/test.mp4"}`),
	}
	jsonData, _ := json.Marshal(reqBody)

	// Use http.NewRequest to allow custom headers
	req, err := http.NewRequest(http.MethodPost, jobURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var jobResp JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return "", err
	}

	return jobResp.ID, nil
}

func checkJobStatus(id string, token string) (string, error) {
	statusURL := fmt.Sprintf("%s/api/v1/jobs/%s", serverURL, id)

	req, err := http.NewRequest(http.MethodGet, statusURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var jobResp JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return "", err
	}

	return jobResp.Status, nil
}
