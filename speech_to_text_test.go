package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateTranscriptUploadsFile(t *testing.T) {
	ctx := context.Background()
	diarize := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/speech-to-text" {
			t.Fatalf("path = %s, want /v1/speech-to-text", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("content-type = %s, want multipart/form-data", mediaType)
		}

		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart reader: %v", err)
		}
		if mr == nil {
			t.Fatal("missing multipart reader")
		}
		if params["boundary"] == "" {
			t.Fatal("missing multipart boundary")
		}

		form, err := mr.ReadForm(1024 * 1024)
		if err != nil {
			t.Fatalf("read form: %v", err)
		}
		defer form.RemoveAll()

		assertFormValue(t, form.Value, "model_id", "scribe_v1")
		assertFormValue(t, form.Value, "language_code", "en")
		assertFormValue(t, form.Value, "timestamps_granularity", "word")
		assertFormValue(t, form.Value, "diarize", "true")
		assertFormValues(t, form.Value, "keyterms", []string{"ElevenLabs", "Scribe"})

		files := form.File["file"]
		if len(files) != 1 {
			t.Fatalf("file parts = %d, want 1", len(files))
		}
		if files[0].Filename != "sample.mp3" {
			t.Fatalf("file name = %q, want sample.mp3", files[0].Filename)
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
		if string(body) != "audio-bytes" {
			t.Fatalf("file body = %q, want audio-bytes", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(map[string]any{
			"text":                 "hello world",
			"language_code":        "en",
			"language_probability": 0.98,
			"words": []map[string]any{
				{
					"text":       "hello",
					"type":       "word",
					"start":      0.0,
					"end":        0.4,
					"speaker_id": "speaker_0",
				},
			},
		})
		if err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID:               "scribe_v1",
		File:                  &File{Name: "sample.mp3", Reader: strings.NewReader("audio-bytes")},
		LanguageCode:          "en",
		TimestampsGranularity: "word",
		Diarize:               &diarize,
		Keyterms:              []string{"ElevenLabs", "Scribe"},
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}

	if transcript.Text != "hello world" {
		t.Fatalf("Text = %q, want hello world", transcript.Text)
	}
	if transcript.LanguageCode != "en" {
		t.Fatalf("LanguageCode = %q, want en", transcript.LanguageCode)
	}
	if transcript.LanguageProbability != 0.98 {
		t.Fatalf("LanguageProbability = %f, want 0.98", transcript.LanguageProbability)
	}
	if len(transcript.Words) != 1 {
		t.Fatalf("Words length = %d, want 1", len(transcript.Words))
	}
	if transcript.Words[0].SpeakerID != "speaker_0" {
		t.Fatalf("Words[0].SpeakerID = %q, want speaker_0", transcript.Words[0].SpeakerID)
	}
}

func TestCreateTranscriptAcceptsSourceURL(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart reader: %v", err)
		}
		form, err := mr.ReadForm(1024 * 1024)
		if err != nil {
			t.Fatalf("read form: %v", err)
		}
		defer form.RemoveAll()

		assertFormValue(t, form.Value, "model_id", "scribe_v1")
		assertFormValue(t, form.Value, "source_url", "https://example.com/audio.mp3")
		if _, ok := form.File["file"]; ok {
			t.Fatal("unexpected file part")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"from url"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID:   "scribe_v1",
		SourceURL: "https://example.com/audio.mp3",
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}
	if transcript.Text != "from url" {
		t.Fatalf("Text = %q, want from url", transcript.Text)
	}
}

func TestCreateTranscriptSendsAdvancedRequestFields(t *testing.T) {
	ctx := context.Background()
	tagAudioEvents := false
	noVerbatim := true
	webhook := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form := readMultipartForm(t, r)

		assertFormValue(t, form.Value, "model_id", "scribe_v1")
		assertFormValue(t, form.Value, "source_url", "https://example.com/audio.mp3")
		assertFormValue(t, form.Value, "num_speakers", "2")
		assertFormValue(t, form.Value, "tag_audio_events", "false")
		assertFormValue(t, form.Value, "no_verbatim", "true")
		assertFormValue(t, form.Value, "webhook", "true")
		assertFormValue(t, form.Value, "webhook_id", "wh_123")

		var metadata map[string]any
		if err := json.Unmarshal([]byte(form.Value["webhook_metadata"][0]), &metadata); err != nil {
			t.Fatalf("webhook_metadata is not JSON: %v", err)
		}
		if metadata["job_id"] != "job_123" || metadata["source"] != "test" {
			t.Fatalf("webhook_metadata = %#v, want job_id and source", metadata)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"accepted"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID:        "scribe_v1",
		SourceURL:      "https://example.com/audio.mp3",
		NumSpeakers:    2,
		TagAudioEvents: &tagAudioEvents,
		NoVerbatim:     &noVerbatim,
		Webhook:        &webhook,
		WebhookID:      "wh_123",
		WebhookMetadata: map[string]any{
			"job_id": "job_123",
			"source": "test",
		},
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}
	if transcript.Text != "accepted" {
		t.Fatalf("Text = %q, want accepted", transcript.Text)
	}
}

func TestCreateTranscriptValidatesRequiredInput(t *testing.T) {
	client := NewClient("test-key")

	_, err := client.CreateTranscript(context.Background(), CreateTranscriptRequest{
		File: &File{Name: "sample.mp3", Reader: strings.NewReader("audio")},
	})
	if err == nil {
		t.Fatal("missing model_id error = nil, want error")
	}

	_, err = client.CreateTranscript(context.Background(), CreateTranscriptRequest{
		ModelID: "scribe_v1",
	})
	if err == nil {
		t.Fatal("missing audio source error = nil, want error")
	}
}

func TestGetTranscript(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.RequestURI() != "/v1/speech-to-text/transcripts/tx_123" {
			t.Fatalf("request uri = %s, want /v1/speech-to-text/transcripts/tx_123", r.URL.RequestURI())
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"stored transcript","language_code":"en"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	transcript, err := client.GetTranscript(ctx, "tx_123")
	if err != nil {
		t.Fatalf("GetTranscript returned error: %v", err)
	}
	if transcript.Text != "stored transcript" {
		t.Fatalf("Text = %q, want stored transcript", transcript.Text)
	}
	if transcript.LanguageCode != "en" {
		t.Fatalf("LanguageCode = %q, want en", transcript.LanguageCode)
	}
}

func TestGetTranscriptEscapesID(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/v1/speech-to-text/transcripts/tx%2F123" {
			t.Fatalf("request uri = %s, want escaped transcript ID", r.URL.RequestURI())
		}
		_, _ = w.Write([]byte(`{"text":"escaped"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	if _, err := client.GetTranscript(ctx, "tx/123"); err != nil {
		t.Fatalf("GetTranscript returned error: %v", err)
	}
}

func TestGetTranscriptValidatesID(t *testing.T) {
	client := NewClient("test-key")

	if _, err := client.GetTranscript(context.Background(), " "); err == nil {
		t.Fatal("GetTranscript error = nil, want missing ID error")
	}
}

func TestDeleteTranscript(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodDelete)
		}
		if r.URL.RequestURI() != "/v1/speech-to-text/transcripts/tx_123" {
			t.Fatalf("request uri = %s, want /v1/speech-to-text/transcripts/tx_123", r.URL.RequestURI())
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	if err := client.DeleteTranscript(ctx, "tx_123"); err != nil {
		t.Fatalf("DeleteTranscript returned error: %v", err)
	}
}

func TestDeleteTranscriptEscapesID(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/v1/speech-to-text/transcripts/tx%2F123" {
			t.Fatalf("request uri = %s, want escaped transcript ID", r.URL.RequestURI())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	if err := client.DeleteTranscript(ctx, "tx/123"); err != nil {
		t.Fatalf("DeleteTranscript returned error: %v", err)
	}
}

func TestDeleteTranscriptValidatesID(t *testing.T) {
	client := NewClient("test-key")

	if err := client.DeleteTranscript(context.Background(), " "); err == nil {
		t.Fatal("DeleteTranscript error = nil, want missing ID error")
	}
}

func TestDeleteTranscriptReturnsAPIError(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":{"message":"rate limited"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	err := client.DeleteTranscript(ctx, "tx_123")
	if err == nil {
		t.Fatal("DeleteTranscript error = nil, want API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusTooManyRequests)
	}
	if apiErr.Message != "rate limited" {
		t.Fatalf("Message = %q, want rate limited", apiErr.Message)
	}
	if !strings.Contains(string(apiErr.Body), "rate limited") {
		t.Fatalf("Body = %q, want to contain rate limited", string(apiErr.Body))
	}
}

func assertFormValue(t *testing.T, values map[string][]string, key, want string) {
	t.Helper()

	got := values[key]
	if len(got) != 1 {
		t.Fatalf("%s values = %v, want one value %q", key, got, want)
	}
	if got[0] != want {
		t.Fatalf("%s = %q, want %q", key, got[0], want)
	}
}

func assertFormValues(t *testing.T, values map[string][]string, key string, want []string) {
	t.Helper()

	got := values[key]
	if len(got) != len(want) {
		t.Fatalf("%s values = %v, want %v", key, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q", key, i, got[i], want[i])
		}
	}
}
