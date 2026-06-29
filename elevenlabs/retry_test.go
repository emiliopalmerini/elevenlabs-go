package elevenlabs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetUserRetriesTransientStatus(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"user_123"}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(3)),
	)

	user, err := client.User.GetUser(ctx)
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if user.UserID != "user_123" {
		t.Fatalf("UserID = %q, want user_123", user.UserID)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestRetryAfterHeaderOverridesBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, `{"detail":{"message":"rate limited"}}`, http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"after_retry_after"}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   time.Hour,
			MaxDelay:    time.Hour,
		}),
	)

	user, err := client.User.GetUser(ctx)
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if user.UserID != "after_retry_after" {
		t.Fatalf("UserID = %q, want after_retry_after", user.UserID)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestWithoutRetriesSendsOneRequest(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithoutRetries(),
	)

	_, err := client.User.GetUser(ctx)
	if err == nil {
		t.Fatal("GetUser error = nil, want API error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}

func TestRetryConfigControlsStatusCodes(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, `{"detail":{"message":"server error"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   time.Nanosecond,
			MaxDelay:    time.Nanosecond,
			StatusCodes: []int{http.StatusTooManyRequests},
		}),
	)

	_, err := client.User.GetUser(ctx)
	if err == nil {
		t.Fatal("GetUser error = nil, want API error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}

func TestContextCancellationStopsRetrying(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, `{"detail":{"message":"server error"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   time.Hour,
			MaxDelay:    time.Hour,
		}),
	)

	_, err := client.User.GetUser(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetUser error = %v, want context deadline exceeded", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}

func TestFinalFailedRetryReturnsAPIError(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, `{"detail":{"message":"still down"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(2)),
	)

	_, err := client.User.GetUser(ctx)
	if err == nil {
		t.Fatal("GetUser error = nil, want API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusInternalServerError)
	}
	if apiErr.Message != "still down" {
		t.Fatalf("Message = %q, want still down", apiErr.Message)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func fastRetryConfig(maxAttempts int) RetryConfig {
	return RetryConfig{
		MaxAttempts: maxAttempts,
		BaseDelay:   time.Nanosecond,
		MaxDelay:    time.Nanosecond,
	}
}
