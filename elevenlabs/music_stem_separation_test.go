package elevenlabs

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSeparateStemsUploadsFileAndReturnsArchive(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/music/stem-separation?output_format=mp3_44100_192" {
			t.Fatalf("request uri = %s, want output_format query", r.URL.RequestURI())
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
		if params["boundary"] == "" {
			t.Fatal("missing multipart boundary")
		}

		form, body := readMultipartFile(t, r)
		assertFormValue(t, form.Value, "stem_variation_id", string(StemVariationTwoStemsV1))
		assertFormValue(t, form.Value, "sign_with_c2pa", "true")

		files := form.File["file"]
		if files[0].Filename != "song.mp3" {
			t.Fatalf("file name = %q, want song.mp3", files[0].Filename)
		}
		if body != "audio-bytes" {
			t.Fatalf("file body = %q, want audio-bytes", body)
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("x-request-id", "req_stems_123")
		_, _ = w.Write([]byte("zip-archive"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Music.SeparateStemsWithResponse(ctx, SeparateStemsRequest{
		File: StemSeparationFile{
			Name:   "song.mp3",
			Reader: strings.NewReader("audio-bytes"),
		},
		OutputFormat:    OutputFormatMP3_44100_192,
		StemVariationID: StemVariationTwoStemsV1,
		SignWithC2PA:    boolPtr(true),
	})
	if err != nil {
		t.Fatalf("SeparateStemsWithResponse returned error: %v", err)
	}
	if string(resp.Data) != "zip-archive" {
		t.Fatalf("Data = %q, want zip-archive", string(resp.Data))
	}
	if resp.RawResponse.Header.Get("x-request-id") != "req_stems_123" {
		t.Fatalf("raw x-request-id = %q, want req_stems_123", resp.RawResponse.Header.Get("x-request-id"))
	}
}

func TestSeparateStemsValidatesFile(t *testing.T) {
	ctx := context.Background()
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("unexpected-archive"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	tests := []struct {
		name string
		in   SeparateStemsRequest
		want string
	}{
		{
			name: "missing file name",
			in: SeparateStemsRequest{
				File: StemSeparationFile{Reader: strings.NewReader("audio")},
			},
			want: "file name is required",
		},
		{
			name: "missing file reader",
			in: SeparateStemsRequest{
				File: StemSeparationFile{Name: "song.mp3"},
			},
			want: "file reader is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Music.SeparateStems(ctx, tt.in)
			if err == nil {
				t.Fatal("SeparateStems error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}

	if requests.Load() != 0 {
		t.Fatalf("server requests = %d, want 0 for validation failures", requests.Load())
	}
}

func TestSeparateStemsReturnsValidationAPIError(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = readMultipartFile(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"loc":["body","file"],"msg":"Invalid audio","type":"value_error"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutRetries())

	_, err := client.Music.SeparateStems(ctx, SeparateStemsRequest{
		File: StemSeparationFile{
			Name:   "song.mp3",
			Reader: strings.NewReader("audio"),
		},
	})
	if err == nil {
		t.Fatal("SeparateStems error = nil, want API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnprocessableEntity)
	}
	if len(apiErr.Validation) != 1 {
		t.Fatalf("Validation length = %d, want 1", len(apiErr.Validation))
	}
	validation := apiErr.Validation[0]
	if validation.Msg != "Invalid audio" {
		t.Fatalf("Validation Msg = %q, want Invalid audio", validation.Msg)
	}
	if !strings.Contains(apiErr.Message, "body.file: Invalid audio") {
		t.Fatalf("Message = %q, want validation location summary", apiErr.Message)
	}
}

func TestSeparateStemsRetriesSeekableFile(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		_, body := readMultipartFile(t, r)
		if body != "audio-bytes" {
			t.Fatalf("file body = %q, want audio-bytes", body)
		}

		if attempt == 1 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("archive-after-retry"))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(2)),
	)

	archive, err := client.Music.SeparateStems(ctx, SeparateStemsRequest{
		File: StemSeparationFile{
			Name:   "song.mp3",
			Reader: strings.NewReader("audio-bytes"),
		},
	})
	if err != nil {
		t.Fatalf("SeparateStems returned error: %v", err)
	}
	if string(archive) != "archive-after-retry" {
		t.Fatalf("archive = %q, want archive-after-retry", string(archive))
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestSeparateStemsDoesNotRetryNonSeekableFile(t *testing.T) {
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

	_, err := client.Music.SeparateStems(ctx, SeparateStemsRequest{
		File: StemSeparationFile{
			Name:   "song.mp3",
			Reader: &nonSeekableReader{r: strings.NewReader("audio-bytes")},
		},
	})
	if err == nil {
		t.Fatal("SeparateStems error = nil, want API error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}
