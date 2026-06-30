package elevenlabs

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVideoToMusicUploadsVideosAndReturnsComposition(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/music/video-to-music?output_format=mp3_44100_192" {
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

		form := readMultipartForm(t, r)
		assertFormValue(t, form.Value, "description", "cinematic background music")
		assertFormValues(t, form.Value, "tags", []string{"upbeat", "cinematic"})
		assertFormValue(t, form.Value, "model_id", string(MusicModelV2))
		assertFormValue(t, form.Value, "sign_with_c2pa", "true")

		videos := form.File["videos"]
		if len(videos) != 2 {
			t.Fatalf("video parts = %d, want 2", len(videos))
		}
		if videos[0].Filename != "intro.mp4" {
			t.Fatalf("videos[0].Filename = %q, want intro.mp4", videos[0].Filename)
		}
		if videos[1].Filename != "outro.mov" {
			t.Fatalf("videos[1].Filename = %q, want outro.mov", videos[1].Filename)
		}
		if got := readUploadedFile(t, videos[0]); got != "intro-video" {
			t.Fatalf("videos[0] body = %q, want intro-video", got)
		}
		if got := readUploadedFile(t, videos[1]); got != "outro-video" {
			t.Fatalf("videos[1] body = %q, want outro-video", got)
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("song-id", "song_video_123")
		_, _ = w.Write([]byte("generated-music"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Music.VideoToMusicWithResponse(ctx, VideoToMusicRequest{
		Videos: []VideoToMusicFile{
			{Name: "intro.mp4", Reader: strings.NewReader("intro-video")},
			{Name: "outro.mov", Reader: strings.NewReader("outro-video")},
		},
		Description:  "cinematic background music",
		Tags:         []string{"upbeat", "cinematic"},
		ModelID:      MusicModelV2,
		OutputFormat: OutputFormatMP3_44100_192,
		SignWithC2PA: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("VideoToMusicWithResponse returned error: %v", err)
	}
	if string(resp.Data.Audio) != "generated-music" {
		t.Fatalf("Audio = %q, want generated-music", string(resp.Data.Audio))
	}
	if resp.Data.SongID != "song_video_123" {
		t.Fatalf("SongID = %q, want song_video_123", resp.Data.SongID)
	}
	if resp.RawResponse.Header.Get("song-id") != "song_video_123" {
		t.Fatalf("raw song-id = %q, want song_video_123", resp.RawResponse.Header.Get("song-id"))
	}
}

func TestVideoToMusicValidatesDocumentedBounds(t *testing.T) {
	ctx := context.Background()
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("unexpected-audio"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	validVideo := VideoToMusicFile{Name: "clip.mp4", Reader: strings.NewReader("video")}
	tests := []struct {
		name string
		in   VideoToMusicRequest
		want string
	}{
		{
			name: "missing videos",
			in:   VideoToMusicRequest{},
			want: "at least one video is required",
		},
		{
			name: "too many videos",
			in:   VideoToMusicRequest{Videos: videoToMusicFiles(11)},
			want: "videos must contain 10 files or fewer",
		},
		{
			name: "missing video name",
			in:   VideoToMusicRequest{Videos: []VideoToMusicFile{{Reader: strings.NewReader("video")}}},
			want: "video 0 name is required",
		},
		{
			name: "missing video reader",
			in:   VideoToMusicRequest{Videos: []VideoToMusicFile{{Name: "clip.mp4"}}},
			want: "video 0 reader is required",
		},
		{
			name: "description max length",
			in:   VideoToMusicRequest{Videos: []VideoToMusicFile{validVideo}, Description: strings.Repeat("a", 1001)},
			want: "description must be 1000 characters or fewer",
		},
		{
			name: "too many tags",
			in:   VideoToMusicRequest{Videos: []VideoToMusicFile{validVideo}, Tags: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}},
			want: "tags must contain 10 values or fewer",
		},
		{
			name: "total known size too large",
			in: VideoToMusicRequest{Videos: []VideoToMusicFile{
				{Name: "one.mp4", Reader: strings.NewReader("video"), SizeBytes: maxVideoToMusicUploadBytes},
				{Name: "two.mp4", Reader: strings.NewReader("video"), SizeBytes: 1},
			}},
			want: "videos total size must be 200MB or fewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Music.VideoToMusic(ctx, tt.in)
			if err == nil {
				t.Fatal("VideoToMusic error = nil, want validation error")
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

func TestVideoToMusicReturnsValidationAPIError(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = readMultipartForm(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"loc":["body","videos"],"msg":"Invalid video","type":"value_error"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutRetries())

	_, err := client.Music.VideoToMusic(ctx, VideoToMusicRequest{
		Videos: []VideoToMusicFile{{Name: "clip.mp4", Reader: strings.NewReader("video")}},
	})
	if err == nil {
		t.Fatal("VideoToMusic error = nil, want API error")
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
	if validation.Msg != "Invalid video" {
		t.Fatalf("Validation Msg = %q, want Invalid video", validation.Msg)
	}
	if !strings.Contains(apiErr.Message, "body.videos: Invalid video") {
		t.Fatalf("Message = %q, want validation location summary", apiErr.Message)
	}
}

func TestVideoToMusicRetriesSeekableVideos(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		form := readMultipartForm(t, r)
		videos := form.File["videos"]
		if len(videos) != 2 {
			t.Fatalf("video parts = %d, want 2", len(videos))
		}
		if got := readUploadedFile(t, videos[0]); got != "first-video" {
			t.Fatalf("videos[0] body = %q, want first-video", got)
		}
		if got := readUploadedFile(t, videos[1]); got != "second-video" {
			t.Fatalf("videos[1] body = %q, want second-video", got)
		}

		if attempt == 1 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("music-after-retry"))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(2)),
	)

	composition, err := client.Music.VideoToMusic(ctx, VideoToMusicRequest{
		Videos: []VideoToMusicFile{
			{Name: "first.mp4", Reader: strings.NewReader("first-video")},
			{Name: "second.mp4", Reader: strings.NewReader("second-video")},
		},
	})
	if err != nil {
		t.Fatalf("VideoToMusic returned error: %v", err)
	}
	if string(composition.Audio) != "music-after-retry" {
		t.Fatalf("Audio = %q, want music-after-retry", string(composition.Audio))
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestVideoToMusicDoesNotRetryNonSeekableVideo(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		form := readMultipartForm(t, r)
		videos := form.File["videos"]
		if len(videos) != 1 {
			t.Fatalf("video parts = %d, want 1", len(videos))
		}
		if got := readUploadedFile(t, videos[0]); got != "video-bytes" {
			t.Fatalf("video body = %q, want video-bytes", got)
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

	_, err := client.Music.VideoToMusic(ctx, VideoToMusicRequest{
		Videos: []VideoToMusicFile{
			{Name: "clip.mp4", Reader: &nonSeekableReader{r: strings.NewReader("video-bytes")}},
		},
	})
	if err == nil {
		t.Fatal("VideoToMusic error = nil, want API error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}

func videoToMusicFiles(n int) []VideoToMusicFile {
	videos := make([]VideoToMusicFile, n)
	for i := range videos {
		videos[i] = VideoToMusicFile{Name: "clip.mp4", Reader: strings.NewReader("video")}
	}
	return videos
}

func readUploadedFile(t *testing.T, header *multipart.FileHeader) string {
	t.Helper()

	file, err := header.Open()
	if err != nil {
		t.Fatalf("open uploaded file: %v", err)
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	return string(body)
}
