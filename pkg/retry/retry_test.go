package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"retry"
)

func TestRetry_Success(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 3

	attempts := 0
	err := retry.Do(context.Background(), cfg, nil, func() error {
		attempts++
		return nil // Success on first try
	})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond

	attempts := 0
	err := retry.Do(context.Background(), cfg, nil, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond

	attempts := 0
	err := retry.Do(context.Background(), cfg, nil, func() error {
		attempts++
		return errors.New("timeout")
	})

	if !errors.Is(err, retry.ErrMaxRetriesExceeded) {
		t.Errorf("expected ErrMaxRetriesExceeded, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 3

	nonRetryableErr := errors.New("permanent failure")

	attempts := 0
	err := retry.Do(context.Background(), cfg, func(err error) bool {
		return false // Not retryable
	}, func() error {
		attempts++
		return nonRetryableErr
	})

	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 5
	cfg.InitialDelay = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	errChan := make(chan error, 1)

	go func() {
		err := retry.Do(ctx, cfg, nil, func() error {
			attempts++
			return errors.New("temporary failure")
		})
		errChan <- err
	}()

	// Cancel after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errChan

	if err == nil {
		t.Error("expected context cancellation error")
	}

	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context error, got: %v", err)
	}

	// Should have attempted at least once
	if attempts < 1 {
		t.Errorf("expected at least 1 attempt, got %d", attempts)
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	cfg := retry.Config{
		MaxAttempts:  4,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	attempts := 0
	start := time.Now()

	err := retry.Do(context.Background(), cfg, nil, func() error {
		attempts++
		return errors.New("temporary failure")
	})

	duration := time.Since(start)

	if err == nil {
		t.Error("expected error after max attempts")
	}

	// Should take at least: 10ms + 20ms + 40ms = 70ms
	expectedMin := 70 * time.Millisecond
	if duration < expectedMin {
		t.Errorf("backoff too short: expected at least %v, got %v", expectedMin, duration)
	}

	if attempts != 4 {
		t.Errorf("expected 4 attempts, got %d", attempts)
	}
}

func TestRetry_MaxDelayRespected(t *testing.T) {
	cfg := retry.Config{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     200 * time.Millisecond,
		Multiplier:   2.0,
	}

	start := time.Now()

	retry.Do(context.Background(), cfg, nil, func() error {
		return errors.New("temporary failure")
	})

	duration := time.Since(start)

	// With multiplier 2.0: 100ms, 200ms, 200ms (capped), 200ms (capped)
	// Total: ~700ms
	expectedMax := 900 * time.Millisecond // Some buffer for execution time
	if duration > expectedMax {
		t.Errorf("backoff too long: expected at most %v, got %v", expectedMax, duration)
	}
}

func TestDefaultRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout error", errors.New("connection timeout"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"permanent error", errors.New("invalid credentials"), false},
		{"not found", errors.New("404 not found"), false},
		{"http 503", errors.New("PBS assign chunks failed: HTTP 503 - busy"), true},
		{"http error 502", errors.New("HTTP error: 502 - bad gateway"), true},
		{"too many requests", errors.New("HTTP 429 - slow down"), true},
		{"deadline exceeded sentinel", context.DeadlineExceeded, true},
		{"http 500 not retried", errors.New("HTTP 500 - internal server error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := retry.DefaultRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

func TestRetry_WithJitter(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond

	// Run multiple times to test jitter variation
	durations := make([]time.Duration, 5)
	for i := 0; i < 5; i++ {
		start := time.Now()
		retry.DoWithJitter(context.Background(), cfg, nil, func() error {
			return errors.New("temporary failure")
		})
		durations[i] = time.Since(start)
	}

	// Check that durations vary (jitter is working)
	allSame := true
	first := durations[0]
	for _, d := range durations[1:] {
		if d != first {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("expected jitter to cause variation in retry durations")
	}
}
