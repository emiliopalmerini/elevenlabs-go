package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestConvertFileStreamsMultipartRequest(t *testing.T) {
	audio := t.TempDir() + "/episode.mp3"
	if err := os.WriteFile(audio, []byte("fake audio"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var sawRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		if r.Method != http.MethodPost || r.URL.Path != "/v1/speech-to-text" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q", got)
		}
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, defaultUserAgent) {
			t.Fatalf("User-Agent = %q", got)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}
		fields := map[string][]string{}
		var fileContent string
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() error = %v", err)
			}
			b, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("ReadAll(part) error = %v", err)
			}
			if part.FormName() == "file" {
				fileContent = string(b)
				if part.FileName() != "episode.mp3" {
					t.Fatalf("file name = %q", part.FileName())
				}
				continue
			}
			fields[part.FormName()] = append(fields[part.FormName()], string(b))
		}
		assertField(t, fields, "model_id", ModelScribeV2)
		assertField(t, fields, "timestamps_granularity", TimestampsWord)
		assertField(t, fields, "language_code", "en")
		assertField(t, fields, "diarize", "true")
		assertField(t, fields, "num_speakers", "2")
		assertField(t, fields, "tag_audio_events", "false")
		assertField(t, fields, "no_verbatim", "true")
		assertField(t, fields, "use_multi_channel", "true")
		assertField(t, fields, "multichannel_output_style", MultichannelCombined)
		assertField(t, fields, "entity_detection", `["pii","phi"]`)
		assertField(t, fields, "entity_redaction", "pii")
		assertField(t, fields, "entity_redaction_mode", EntityRedactionRedacted)
		if got := strings.Join(fields["keyterms"], ","); got != "Emilio,Podscribe" {
			t.Fatalf("keyterms = %q", got)
		}
		if fileContent != "fake audio" {
			t.Fatalf("file content = %q", fileContent)
		}
		_, _ = w.Write([]byte(`{"language_code":"en","text":"Hello","words":[],"transcription_id":"tx_123"}`))
	}))
	defer server.Close()

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithUserAgent("podscribe-test/1.0"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var progress []UploadProgress
	resp, err := client.SpeechToText.ConvertFile(context.Background(), audio, &SpeechToTextRequest{
		ModelID:                 ModelScribeV2,
		LanguageCode:            "en",
		Diarize:                 Bool(true),
		NumSpeakers:             Int(2),
		Keyterms:                []string{"Emilio", "Podscribe"},
		NoVerbatim:              Bool(true),
		TagAudioEvents:          Bool(false),
		TimestampsGranularity:   TimestampsWord,
		UseMultiChannel:         Bool(true),
		MultichannelOutputStyle: MultichannelCombined,
		EntityDetection:         EntitySelector{EntityCategoryPII, EntityCategoryPHI},
		EntityRedaction:         EntitySelector{EntityCategoryPII},
		EntityRedactionMode:     EntityRedactionRedacted,
		OnUploadProgress: func(update UploadProgress) {
			progress = append(progress, update)
		},
	})
	if err != nil {
		t.Fatalf("ConvertFile() error = %v", err)
	}
	if !sawRequest {
		t.Fatal("server did not receive request")
	}
	if resp.TranscriptionID != "tx_123" {
		t.Fatalf("transcription ID = %q", resp.TranscriptionID)
	}
	if len(progress) == 0 {
		t.Fatal("upload progress was not reported")
	}
	lastProgress := progress[len(progress)-1]
	if lastProgress.SentBytes != int64(len("fake audio")) || lastProgress.TotalBytes != int64(len("fake audio")) {
		t.Fatalf("last progress = %+v, want sent and total %d", lastProgress, len("fake audio"))
	}
}

func TestConvertSourceURLSendsFieldsAndEnableLoggingQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("enable_logging"); got != "false" {
			t.Fatalf("enable_logging = %q", got)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}
		fields := map[string][]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() error = %v", err)
			}
			if part.FormName() == "file" {
				t.Fatal("source URL request should not include file part")
			}
			b, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("ReadAll(part) error = %v", err)
			}
			fields[part.FormName()] = append(fields[part.FormName()], string(b))
		}
		assertField(t, fields, "model_id", ModelScribeV2)
		assertField(t, fields, "source_url", "https://example.com/audio.mp3")
		assertField(t, fields, "temperature", "0.3")
		assertField(t, fields, "seed", "42")
		_, _ = w.Write([]byte(`{"text":"Hello from URL"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	resp, err := client.SpeechToText.Convert(context.Background(), &SpeechToTextRequest{
		ModelID:       ModelScribeV2,
		SourceURL:     "https://example.com/audio.mp3",
		EnableLogging: Bool(false),
		Temperature:   Float64(0.3),
		Seed:          Int(42),
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if resp.Text != "Hello from URL" {
		t.Fatalf("text = %q", resp.Text)
	}
}

func TestSubmitWebhookFileSendsMetadata(t *testing.T) {
	audio := t.TempDir() + "/episode.mp3"
	if err := os.WriteFile(audio, []byte("fake audio"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var fields map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}
		fields = map[string][]string{}
		var sawFile bool
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() error = %v", err)
			}
			b, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("ReadAll(part) error = %v", err)
			}
			if part.FormName() == "file" {
				sawFile = true
				continue
			}
			fields[part.FormName()] = append(fields[part.FormName()], string(b))
		}
		if !sawFile {
			t.Fatal("multipart request did not include file")
		}
		_, _ = w.Write([]byte(`{"message":"accepted","request_id":"req_123","transcription_id":"tx_123"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	resp, err := client.SpeechToText.SubmitWebhookFile(context.Background(), audio, &SpeechToTextRequest{
		ModelID:               ModelScribeV2,
		TimestampsGranularity: TimestampsWord,
		WebhookID:             "wh_123",
		WebhookMetadata: map[string]any{
			"job": "job_123",
		},
	})
	if err != nil {
		t.Fatalf("SubmitWebhookFile() error = %v", err)
	}
	if resp.RequestID != "req_123" || resp.TranscriptionID == nil || *resp.TranscriptionID != "tx_123" {
		t.Fatalf("webhook response = %+v", resp)
	}
	assertField(t, fields, "webhook", "true")
	assertField(t, fields, "webhook_id", "wh_123")
	var metadata map[string]string
	if err := json.Unmarshal([]byte(fields["webhook_metadata"][0]), &metadata); err != nil {
		t.Fatalf("webhook_metadata is not JSON: %v", err)
	}
	if metadata["job"] != "job_123" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestGetDeleteUserAndModels(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/speech-to-text/transcripts/tx_123":
			_, _ = w.Write([]byte(`{"language_code":"en","text":"Stored","words":[]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/speech-to-text/transcripts/tx_123":
			_, _ = w.Write([]byte(`{"deleted":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user":
			_, _ = w.Write([]byte(`{"user_id":"user_123","seat_type":"workspace_admin"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			_, _ = w.Write([]byte(`[{"model_id":"scribe_v2","name":"Scribe v2","can_do_text_to_speech":true}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server)
	if _, err := client.SpeechToText.GetTranscript(context.Background(), "tx_123"); err != nil {
		t.Fatalf("GetTranscript() error = %v", err)
	}
	if err := client.SpeechToText.DeleteTranscript(context.Background(), "tx_123"); err != nil {
		t.Fatalf("DeleteTranscript() error = %v", err)
	}
	user, err := client.User.Get(context.Background())
	if err != nil {
		t.Fatalf("User.Get() error = %v", err)
	}
	if user.UserID != "user_123" {
		t.Fatalf("user = %+v", user)
	}
	models, err := client.Models.List(context.Background())
	if err != nil {
		t.Fatalf("Models.List() error = %v", err)
	}
	if len(models) != 1 || models[0].ModelID != ModelScribeV2 || !models[0].CanDoTextToSpeech {
		t.Fatalf("models = %+v", models)
	}
	want := []string{
		"GET /v1/speech-to-text/transcripts/tx_123",
		"DELETE /v1/speech-to-text/transcripts/tx_123",
		"GET /v1/user",
		"GET /v1/models",
	}
	if strings.Join(requests, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}

func TestValidation(t *testing.T) {
	client, err := NewClient(WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tests := []struct {
		name string
		req  *SpeechToTextRequest
	}{
		{name: "missing model", req: &SpeechToTextRequest{SourceURL: "https://example.com/audio.mp3"}},
		{name: "missing source", req: &SpeechToTextRequest{ModelID: ModelScribeV2}},
		{name: "too many sources", req: &SpeechToTextRequest{ModelID: ModelScribeV2, Audio: strings.NewReader("x"), SourceURL: "https://example.com/audio.mp3"}},
		{name: "bad speaker count", req: &SpeechToTextRequest{ModelID: ModelScribeV2, SourceURL: "https://example.com/audio.mp3", NumSpeakers: Int(33)}},
		{name: "bad seed", req: &SpeechToTextRequest{ModelID: ModelScribeV2, SourceURL: "https://example.com/audio.mp3", Seed: Int(-1)}},
		{name: "bad temperature", req: &SpeechToTextRequest{ModelID: ModelScribeV2, SourceURL: "https://example.com/audio.mp3", Temperature: Float64(2.1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := client.SpeechToText.Convert(context.Background(), tt.req); err == nil {
				t.Fatal("Convert() error = nil, want validation error")
			}
		})
	}
}

func TestConvertFileMissingPathFailsBeforeNetwork(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.SpeechToText.ConvertFile(context.Background(), t.TempDir()+"/missing.mp3", &SpeechToTextRequest{
		ModelID: ModelScribeV2,
	})
	if err == nil {
		t.Fatal("ConvertFile() error = nil, want missing file error")
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient(); err == nil {
		t.Fatal("NewClient() error = nil, want missing API key")
	}
	if _, err := NewClient(WithAPIKey("test-key"), WithBaseURL("://bad")); err == nil {
		t.Fatal("NewClient() error = nil, want invalid base URL")
	}
	if _, err := NewClient(WithAPIKey("test-key"), WithHTTPClient(nil)); err == nil {
		t.Fatal("NewClient() error = nil, want nil HTTP client")
	}
	if _, err := NewClient(WithAPIKey("test-key"), WithRetryPolicy(RetryPolicy{})); err == nil {
		t.Fatal("NewClient() error = nil, want invalid retry policy")
	}
}

func TestRetryPolicyRetriesReplayableSourceURLRateLimit(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("retry-after", "0")
			http.Error(w, `{"detail":{"message":"Too many requests"}}`, http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer server.Close()

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryPolicy(DefaultRetryPolicy()),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	resp, err := client.SpeechToText.Convert(context.Background(), &SpeechToTextRequest{
		ModelID:   ModelScribeV2,
		SourceURL: "https://example.com/audio.mp3",
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if resp.Text != "ok" || attempts != 2 {
		t.Fatalf("text=%q attempts=%d", resp.Text, attempts)
	}
}

func TestRetryPolicyDoesNotRetryOneShotReaderUpload(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, `{"detail":{"message":"Too many requests"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryPolicy(DefaultRetryPolicy()),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.SpeechToText.Convert(context.Background(), &SpeechToTextRequest{
		ModelID:  ModelScribeV2,
		Audio:    strings.NewReader("audio"),
		FileName: "audio.mp3",
		FileSize: 5,
	})
	if err == nil {
		t.Fatal("Convert() error = nil, want API error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestGetRetriesConnectionErrors(t *testing.T) {
	var attempts int
	client, err := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL("https://api.example"),
		WithRetryPolicy(RetryPolicy{
			MaxAttempts:           2,
			RetryConnectionErrors: true,
		}),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
			}
			return jsonResponse(req, http.StatusOK, `[]`), nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Models.List(context.Background()); err != nil {
		t.Fatalf("Models.List() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestChunksAdaptsSingleTranscript(t *testing.T) {
	resp := TranscriptResponse{Text: "Hello", Words: []Word{{Text: "Hello"}}}
	chunks := resp.Chunks()
	if len(chunks) != 1 || chunks[0].Text != "Hello" || chunks[0].Words[0].Text != "Hello" {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func assertField(t *testing.T, fields map[string][]string, name, want string) {
	t.Helper()
	got := fields[name]
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s = %#v, want %q", name, got, want)
	}
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	client, err := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(req *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
