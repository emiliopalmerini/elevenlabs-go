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

func TestCreateTranscriptParsesDocumentedResponseFields(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = readMultipartForm(t, r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"language_code": "en",
			"language_probability": 0.98,
			"text": "Hello!",
			"transcription_id": "tx_123",
			"audio_duration_secs": 1.25,
			"entities": [
				{"text":"Emilio","entity_type":"person_name","start_char":0,"end_char":6}
			],
			"words": [
				{
					"text": "Hello",
					"start": 0,
					"end": 0.5,
					"type": "word",
					"speaker_id": "speaker_1",
					"logprob": -0.124,
					"channel_index": 1,
					"characters": [
						{"text":"H","start":0,"end":0.1}
					]
				},
				{"text":"!","start":null,"end":null,"type":"spacing","logprob":-0.2}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID:   "scribe_v2",
		SourceURL: "https://example.com/audio.mp3",
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}
	if transcript.TranscriptionID != "tx_123" {
		t.Fatalf("TranscriptionID = %q, want tx_123", transcript.TranscriptionID)
	}
	if transcript.AudioDurationSecs == nil || *transcript.AudioDurationSecs != 1.25 {
		t.Fatalf("AudioDurationSecs = %v, want 1.25", transcript.AudioDurationSecs)
	}
	if len(transcript.Entities) != 1 || transcript.Entities[0].EntityType != "person_name" {
		t.Fatalf("Entities = %#v, want detected person_name entity", transcript.Entities)
	}
	word := transcript.Words[0]
	if word.Start == nil || *word.Start != 0 {
		t.Fatalf("Words[0].Start = %v, want 0", word.Start)
	}
	if word.End == nil || *word.End != 0.5 {
		t.Fatalf("Words[0].End = %v, want 0.5", word.End)
	}
	if word.Logprob != -0.124 {
		t.Fatalf("Words[0].Logprob = %f, want -0.124", word.Logprob)
	}
	if word.ChannelIndex == nil || *word.ChannelIndex != 1 {
		t.Fatalf("Words[0].ChannelIndex = %v, want 1", word.ChannelIndex)
	}
	if len(word.Characters) != 1 || word.Characters[0].Text != "H" {
		t.Fatalf("Words[0].Characters = %#v, want H character timing", word.Characters)
	}
	if transcript.Words[1].Start != nil || transcript.Words[1].End != nil {
		t.Fatalf("Words[1] times = %v/%v, want nil", transcript.Words[1].Start, transcript.Words[1].End)
	}
}

func TestCreateTranscriptParsesMultichannelResponse(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = readMultipartForm(t, r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"transcription_id": "tx_multi",
			"audio_duration_secs": 12.5,
			"transcripts": [
				{
					"language_code": "en",
					"language_probability": 0.99,
					"text": "Channel zero",
					"channel_index": 0,
					"words": [{"text":"Channel","type":"word","logprob":-0.1,"channel_index":0}]
				},
				{
					"language_code": "en",
					"language_probability": 0.97,
					"text": "Channel one",
					"channel_index": 1,
					"words": [{"text":"Channel","type":"word","logprob":-0.2,"channel_index":1}]
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID:         "scribe_v2",
		SourceURL:       "https://example.com/audio.mp3",
		UseMultiChannel: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("CreateTranscript returned error: %v", err)
	}
	if len(transcript.Transcripts) != 2 {
		t.Fatalf("Transcripts length = %d, want 2", len(transcript.Transcripts))
	}
	if transcript.Transcripts[1].ChannelIndex == nil || *transcript.Transcripts[1].ChannelIndex != 1 {
		t.Fatalf("Transcripts[1].ChannelIndex = %v, want 1", transcript.Transcripts[1].ChannelIndex)
	}
	if transcript.Transcripts[1].Words[0].ChannelIndex == nil || *transcript.Transcripts[1].Words[0].ChannelIndex != 1 {
		t.Fatalf("Transcripts[1].Words[0].ChannelIndex = %v, want 1", transcript.Transcripts[1].Words[0].ChannelIndex)
	}
}

func TestSubmitTranscriptWebhookReturnsAcceptance(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form := readMultipartForm(t, r)
		assertFormValue(t, form.Value, "webhook", "true")
		assertFormValue(t, form.Value, "webhook_id", "wh_123")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Request accepted. Transcription result will be sent to the webhook.",
			"request_id": "req_123",
			"transcription_id": "tx_123"
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	response, err := client.SubmitTranscriptWebhook(ctx, CreateTranscriptRequest{
		ModelID:   "scribe_v2",
		SourceURL: "https://example.com/audio.mp3",
		WebhookID: "wh_123",
	})
	if err != nil {
		t.Fatalf("SubmitTranscriptWebhook returned error: %v", err)
	}
	if response.RequestID != "req_123" {
		t.Fatalf("RequestID = %q, want req_123", response.RequestID)
	}
	if response.TranscriptionID == nil || *response.TranscriptionID != "tx_123" {
		t.Fatalf("TranscriptionID = %v, want tx_123", response.TranscriptionID)
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

func TestCreateTranscriptSendsRemainingRequestFields(t *testing.T) {
	ctx := context.Background()
	useSpeakerLibrary := true
	detectSpeakerRoles := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form := readMultipartForm(t, r)

		assertFormValue(t, form.Value, "model_id", "scribe_v2")
		assertFormValue(t, form.Value, "source_url", "https://example.com/audio.pcm")
		assertFormValue(t, form.Value, "diarization_threshold", "0.33")
		assertFormValue(t, form.Value, "file_format", "pcm_s16le_16")
		assertFormValue(t, form.Value, "temperature", "0.7")
		assertFormValue(t, form.Value, "seed", "1234")
		assertFormValues(t, form.Value, "entity_detection", []string{"pii", "phi"})
		assertFormValue(t, form.Value, "use_speaker_library", "true")
		assertFormValue(t, form.Value, "detect_speaker_roles", "true")
		assertFormValues(t, form.Value, "entity_redaction", []string{"email_address", "phone_number"})
		assertFormValue(t, form.Value, "entity_redaction_mode", "entity_type")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"accepted"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	transcript, err := client.CreateTranscript(ctx, CreateTranscriptRequest{
		ModelID:               "scribe_v2",
		SourceURL:             "https://example.com/audio.pcm",
		DiarizationThreshold:  floatPtr(0.33),
		FileFormat:            "pcm_s16le_16",
		Temperature:           floatPtr(0.7),
		Seed:                  intPtr(1234),
		EntityDetection:       []string{"pii", "phi"},
		UseSpeakerLibrary:     &useSpeakerLibrary,
		DetectSpeakerRoles:    &detectSpeakerRoles,
		EntityRedaction:       []string{"email_address", "phone_number"},
		EntityRedactionMode:   "entity_type",
		TimestampsGranularity: "word",
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

func boolPtr(v bool) *bool {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
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
