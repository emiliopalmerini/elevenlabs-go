package elevenlabs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSpeechEngineListSendsQueryAndParsesPage(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/speech-engine" {
			t.Fatalf("path = %s, want /v1/speech-engine", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("page_size") != "10" || query.Get("search") != "orders" || query.Get("sort_direction") != "asc" || query.Get("sort_by") != "name" || query.Get("cursor") != "cursor_1" {
			t.Fatalf("query = %s, want list filters", r.URL.RawQuery)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"speech_engines": [
				{
					"speech_engine_id": "seng_123",
					"name": "Orders",
					"created_at_unix_secs": 1700000000,
					"tags": ["prod"]
				}
			],
			"next_cursor": "cursor_2",
			"has_more": true
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	pageSize := 10

	page, err := client.SpeechEngine.List(ctx, SpeechEngineListRequest{
		PageSize:      &pageSize,
		Search:        "orders",
		SortDirection: "asc",
		SortBy:        "name",
		Cursor:        "cursor_1",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page.SpeechEngines) != 1 || page.SpeechEngines[0].SpeechEngineID != "seng_123" {
		t.Fatalf("SpeechEngines = %#v, want seng_123", page.SpeechEngines)
	}
	if page.NextCursor == nil || *page.NextCursor != "cursor_2" || !page.HasMore {
		t.Fatalf("pagination = %#v %v, want cursor_2 true", page.NextCursor, page.HasMore)
	}
}

func TestSpeechEngineCreateSendsJSONAndReturnsResource(t *testing.T) {
	ctx := context.Background()
	latency := 2

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/speech-engine" {
			t.Fatalf("path = %s, want /v1/speech-engine", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body SpeechEngineCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Name != "Orders" {
			t.Fatalf("Name = %q, want Orders", body.Name)
		}
		if body.SpeechEngine.WSURL != "wss://example.com/upstream" {
			t.Fatalf("WSURL = %q, want upstream URL", body.SpeechEngine.WSURL)
		}
		if body.TTS == nil || body.TTS.OptimizeStreamingLatency == nil || *body.TTS.OptimizeStreamingLatency != latency {
			t.Fatalf("TTS = %#v, want latency 2", body.TTS)
		}

		w.Header().Set("X-Request-ID", "req_create")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"speech_engine_id": "seng_123",
			"name": "Orders",
			"speech_engine": {"ws_url": "wss://example.com/upstream"},
			"tts": {"model_id": "eleven_flash_v2_5"}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.SpeechEngine.CreateWithResponse(ctx, SpeechEngineCreateRequest{
		Name: "Orders",
		SpeechEngine: SpeechEngineConfig{
			WSURL: "wss://example.com/upstream",
		},
		TTS: &SpeechEngineTTSConfig{
			OptimizeStreamingLatency: &latency,
		},
	})
	if err != nil {
		t.Fatalf("CreateWithResponse returned error: %v", err)
	}
	if resp.Data.SpeechEngineID != "seng_123" || resp.Data.TTS == nil || resp.Data.TTS.ModelID != "eleven_flash_v2_5" {
		t.Fatalf("Data = %#v, want created speech engine", resp.Data)
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_create" {
		t.Fatalf("X-Request-ID = %q, want req_create", resp.RawResponse.Header.Get("X-Request-ID"))
	}
}

func TestSpeechEngineGetUpdateAndDeleteUseEscapedID(t *testing.T) {
	ctx := context.Background()
	var seen atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := seen.Add(1)
		if r.URL.EscapedPath() != "/v1/speech-engine/seng_abc%2F123" {
			t.Fatalf("path = %s, want escaped speech engine id", r.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")

		switch n {
		case 1:
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			_, _ = w.Write([]byte(`{"speech_engine_id":"seng_abc/123","name":"Orders","speech_engine":{"ws_url":"wss://old"}}`))
		case 2:
			if r.Method != http.MethodPatch {
				t.Fatalf("method = %s, want PATCH", r.Method)
			}
			var body SpeechEngineUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode update request: %v", err)
			}
			if body.Name != "Support" {
				t.Fatalf("Name = %q, want Support", body.Name)
			}
			_, _ = w.Write([]byte(`{"speech_engine_id":"seng_abc/123","name":"Support","speech_engine":{"ws_url":"wss://old"}}`))
		case 3:
			if r.Method != http.MethodDelete {
				t.Fatalf("method = %s, want DELETE", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %d", n)
		}
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	got, err := client.SpeechEngine.Get(ctx, "seng_abc/123")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.SpeechEngineID != "seng_abc/123" {
		t.Fatalf("SpeechEngineID = %q, want seng_abc/123", got.SpeechEngineID)
	}

	updated, err := client.SpeechEngine.Update(ctx, "seng_abc/123", SpeechEngineUpdateRequest{Name: "Support"})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Name != "Support" {
		t.Fatalf("updated Name = %q, want Support", updated.Name)
	}

	if err := client.SpeechEngine.Delete(ctx, "seng_abc/123"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if seen.Load() != 3 {
		t.Fatalf("requests = %d, want 3", seen.Load())
	}
}

func TestSpeechEngineMethodsValidateRequiredFields(t *testing.T) {
	ctx := context.Background()
	client := NewClient("test-key")

	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "create ws url",
			run: func() error {
				_, err := client.SpeechEngine.Create(ctx, SpeechEngineCreateRequest{})
				return err
			},
			want: "ws_url is required",
		},
		{
			name: "get id",
			run: func() error {
				_, err := client.SpeechEngine.Get(ctx, " ")
				return err
			},
			want: "speech engine id is required",
		},
		{
			name: "update id",
			run: func() error {
				_, err := client.SpeechEngine.Update(ctx, "", SpeechEngineUpdateRequest{Name: "x"})
				return err
			},
			want: "speech engine id is required",
		},
		{
			name: "delete id",
			run: func() error {
				return client.SpeechEngine.Delete(ctx, "")
			},
			want: "speech engine id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}
}
