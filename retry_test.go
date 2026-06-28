package elevenlabs

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetTranscriptRetriesTransientStatus(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"retried transcript"}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(3)),
	)

	transcript, err := client.GetTranscript(ctx, "tx_123")
	if err != nil {
		t.Fatalf("GetTranscript returned error: %v", err)
	}
	if transcript.Text != "retried transcript" {
		t.Fatalf("Text = %q, want retried transcript", transcript.Text)
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
		_, _ = w.Write([]byte(`{"text":"after retry-after"}`))
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

	transcript, err := client.GetTranscript(ctx, "tx_123")
	if err != nil {
		t.Fatalf("GetTranscript returned error: %v", err)
	}
	if transcript.Text != "after retry-after" {
		t.Fatalf("Text = %q, want after retry-after", transcript.Text)
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

	err := client.DeleteTranscript(ctx, "tx_123")
	if err == nil {
		t.Fatal("DeleteTranscript error = nil, want API error")
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

	err := client.DeleteTranscript(ctx, "tx_123")
	if err == nil {
		t.Fatal("DeleteTranscript error = nil, want API error")
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

	_, err := client.GetTranscript(ctx, "tx_123")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetTranscript error = %v, want context deadline exceeded", err)
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

	_, err := client.GetTranscript(ctx, "tx_123")
	if err == nil {
		t.Fatal("GetTranscript error = nil, want API error")
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

func TestCreateTranscriptRetriesSourceURL(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		form := readMultipartForm(t, r)
		assertFormValue(t, form.Value, "model_id", "scribe_v1")
		assertFormValue(t, form.Value, "source_url", "https://example.com/audio.mp3")
		if _, ok := form.File["file"]; ok {
			t.Fatal("unexpected file part")
		}

		if attempt == 1 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"from url after retry"}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(2)),
	)

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID:   "scribe_v1",
		SourceURL: "https://example.com/audio.mp3",
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}
	if transcript.Text != "from url after retry" {
		t.Fatalf("Text = %q, want from url after retry", transcript.Text)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestCreateTranscriptDoesNotRetryNonSeekableFile(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		_, body := readMultipartFile(t, r)
		if body != "audio-bytes" {
			t.Fatalf("file body = %q, want audio-bytes", body)
		}

		http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(3)),
	)

	_, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID: "scribe_v1",
		File: &File{
			Name:   "sample.mp3",
			Reader: &nonSeekableReader{r: strings.NewReader("audio-bytes")},
		},
	})
	if err == nil {
		t.Fatal("CreateTranscript error = nil, want API error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}

func TestCreateTranscriptRetriesSeekableFile(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		form, body := readMultipartFile(t, r)
		assertFormValue(t, form.Value, "model_id", "scribe_v1")
		if body != "audio-bytes" {
			t.Fatalf("file body = %q, want audio-bytes", body)
		}

		if attempt == 1 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"from seekable file after retry"}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(2)),
	)

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID: "scribe_v1",
		File:    &File{Name: "sample.mp3", Reader: strings.NewReader("audio-bytes")},
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}
	if transcript.Text != "from seekable file after retry" {
		t.Fatalf("Text = %q, want from seekable file after retry", transcript.Text)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestCreateTranscriptProgressReportsRetryAttempts(t *testing.T) {
	ctx := context.Background()
	audio := "audio-bytes"
	var attempts atomic.Int32
	var progress []UploadProgress

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		_, body := readMultipartFile(t, r)
		if body != audio {
			t.Fatalf("file body = %q, want %q", body, audio)
		}

		if attempt == 1 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"from seekable file after retry"}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(2)),
	)

	_, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID: "scribe_v1",
		File: &File{
			Name:      "sample.mp3",
			Reader:    strings.NewReader(audio),
			SizeBytes: int64(len(audio)),
		},
		OnUploadProgress: func(update UploadProgress) {
			progress = append(progress, update)
		},
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}

	var doneAttempts []int
	initialByAttempt := map[int]UploadProgress{}
	for _, update := range progress {
		if update.SentBytes == 0 {
			initialByAttempt[update.Attempt] = update
		}
		if update.Done {
			doneAttempts = append(doneAttempts, update.Attempt)
		}
	}
	if _, ok := initialByAttempt[1]; !ok {
		t.Fatalf("progress events = %v, want initial event for attempt 1", progress)
	}
	if _, ok := initialByAttempt[2]; !ok {
		t.Fatalf("progress events = %v, want initial event for attempt 2", progress)
	}
	if len(doneAttempts) != 2 || doneAttempts[0] != 1 || doneAttempts[1] != 2 {
		t.Fatalf("done attempts = %v, want [1 2]", doneAttempts)
	}
}

type nonSeekableReader struct {
	r *strings.Reader
}

func (r *nonSeekableReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func fastRetryConfig(maxAttempts int) RetryConfig {
	return RetryConfig{
		MaxAttempts: maxAttempts,
		BaseDelay:   time.Nanosecond,
		MaxDelay:    time.Nanosecond,
	}
}

func readMultipartForm(t *testing.T, r *http.Request) *multipartForm {
	t.Helper()

	mr, err := r.MultipartReader()
	if err != nil {
		t.Fatalf("multipart reader: %v", err)
	}
	form, err := mr.ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	t.Cleanup(func() {
		_ = form.RemoveAll()
	})

	return &multipartForm{
		Value: form.Value,
		File:  form.File,
	}
}

func readMultipartFile(t *testing.T, r *http.Request) (*multipartForm, string) {
	t.Helper()

	form := readMultipartForm(t, r)
	files := form.File["file"]
	if len(files) != 1 {
		t.Fatalf("file parts = %d, want 1", len(files))
	}

	file, err := files[0].Open()
	if err != nil {
		t.Fatalf("open uploaded file: %v", err)
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}

	return form, string(body)
}

type multipartForm struct {
	Value map[string][]string
	File  map[string][]*multipart.FileHeader
}
