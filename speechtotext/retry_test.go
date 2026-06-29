package speechtotext

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	elevenlabs "github.com/emiliopalmerini/elevenlabs-go"
)

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
		elevenlabs.WithBaseURL(server.URL),
		elevenlabs.WithHTTPClient(server.Client()),
		elevenlabs.WithRetryConfig(fastRetryConfig(2)),
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
		elevenlabs.WithBaseURL(server.URL),
		elevenlabs.WithHTTPClient(server.Client()),
		elevenlabs.WithRetryConfig(fastRetryConfig(3)),
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
		elevenlabs.WithBaseURL(server.URL),
		elevenlabs.WithHTTPClient(server.Client()),
		elevenlabs.WithRetryConfig(fastRetryConfig(2)),
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
		elevenlabs.WithBaseURL(server.URL),
		elevenlabs.WithHTTPClient(server.Client()),
		elevenlabs.WithRetryConfig(fastRetryConfig(2)),
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

func fastRetryConfig(maxAttempts int) elevenlabs.RetryConfig {
	return elevenlabs.RetryConfig{
		MaxAttempts: maxAttempts,
		BaseDelay:   time.Nanosecond,
		MaxDelay:    time.Nanosecond,
	}
}
