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

func TestVoicesListSharedSendsQueryAndParsesPage(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/shared-voices" {
			t.Fatalf("path = %s, want /v1/shared-voices", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		query := r.URL.Query()
		if query.Get("page_size") != "10" ||
			query.Get("category") != "professional" ||
			query.Get("gender") != "Female" ||
			query.Get("age") != "young" ||
			query.Get("accent") != "american" ||
			query.Get("language") != "en" ||
			query.Get("locale") != "en-US" ||
			query.Get("search") != "calm" ||
			query.Get("featured") != "false" ||
			query.Get("min_notice_period_days") != "7" ||
			query.Get("include_custom_rates") != "true" ||
			query.Get("include_live_moderated") != "false" ||
			query.Get("reader_app_enabled") != "true" ||
			query.Get("owner_id") != "owner_123" ||
			query.Get("sort") != "trending" ||
			query.Get("page") != "2" {
			t.Fatalf("query = %s, want shared voice filters", r.URL.RawQuery)
		}
		assertVoiceQueryValues(t, query["use_cases"], []string{"characters_animation", "narration"})
		assertVoiceQueryValues(t, query["descriptives"], []string{"calm", "strong"})

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"voices": [
				{
					"public_owner_id": "owner_123",
					"voice_id": "sB1b5zUrxQVAFl2PhZFp",
					"date_unix": 1714423232,
					"name": "Alita",
					"accent": "american",
					"gender": "Female",
					"age": "young",
					"descriptive": "calm",
					"use_case": "characters_animation",
					"category": "professional",
					"language": "en",
					"locale": "en-US",
					"description": "Perfectly calm voice.",
					"preview_url": "https://example.com/preview.mp3",
					"usage_character_count_1y": 12852,
					"usage_character_count_7d": 42,
					"play_api_usage_character_count_1y": 500,
					"cloned_by_count": 11,
					"rate": 1.5,
					"fiat_rate": 0.12,
					"free_users_allowed": true,
					"live_moderation_enabled": false,
					"featured": true,
					"verified_languages": [
						{
							"language": "en",
							"model_id": "eleven_multilingual_v2",
							"accent": "american",
							"locale": "en-US",
							"preview_url": "https://example.com/verified.mp3"
						}
					],
					"notice_period": 7,
					"instagram_username": "alita_voice",
					"twitter_username": "alita",
					"youtube_username": "alita_channel",
					"tiktok_username": "alita_tok",
					"image_url": "https://example.com/image.png",
					"is_added_by_user": true,
					"is_bookmarked": false
				}
			],
			"has_more": true,
			"total_count": 12,
			"last_sort_id": "sort_123"
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	pageSize := 10
	featured := false
	noticePeriod := 7
	includeCustomRates := true
	includeLiveModerated := false
	readerAppEnabled := true
	page := 2

	resp, err := client.Voices.ListShared(ctx, ListSharedVoicesRequest{
		Accent:               "american",
		Age:                  "young",
		Category:             ListSharedVoicesCategoryProfessional,
		Descriptives:         []string{"calm", "strong"},
		Featured:             &featured,
		Gender:               "Female",
		IncludeCustomRates:   &includeCustomRates,
		IncludeLiveModerated: &includeLiveModerated,
		Language:             "en",
		Locale:               "en-US",
		MinNoticePeriodDays:  &noticePeriod,
		OwnerID:              "owner_123",
		Page:                 &page,
		PageSize:             &pageSize,
		ReaderAppEnabled:     &readerAppEnabled,
		Search:               "calm",
		Sort:                 ListSharedVoicesSortTrending,
		UseCases:             []string{"characters_animation", "narration"},
	})
	if err != nil {
		t.Fatalf("ListShared returned error: %v", err)
	}
	if len(resp.Voices) != 1 {
		t.Fatalf("voices length = %d, want 1", len(resp.Voices))
	}

	voice := resp.Voices[0]
	if voice.VoiceID != "sB1b5zUrxQVAFl2PhZFp" || voice.Name != "Alita" || voice.Category != SharedVoiceCategoryProfessional {
		t.Fatalf("voice = %#v, want parsed shared voice", voice)
	}
	if voice.Language == nil || *voice.Language != "en" || voice.Locale == nil || *voice.Locale != "en-US" {
		t.Fatalf("language/locale = %#v/%#v, want en/en-US", voice.Language, voice.Locale)
	}
	if voice.Rate == nil || *voice.Rate != 1.5 || voice.FiatRate == nil || *voice.FiatRate != 0.12 {
		t.Fatalf("rates = %#v/%#v, want parsed rates", voice.Rate, voice.FiatRate)
	}
	if len(voice.VerifiedLanguages) != 1 || voice.VerifiedLanguages[0].ModelID != "eleven_multilingual_v2" {
		t.Fatalf("verified languages = %#v, want multilingual language", voice.VerifiedLanguages)
	}
	if voice.IsAddedByUser == nil || !*voice.IsAddedByUser || voice.IsBookmarked == nil || *voice.IsBookmarked {
		t.Fatalf("user flags = %#v/%#v, want true/false", voice.IsAddedByUser, voice.IsBookmarked)
	}
	if !resp.HasMore || resp.TotalCount != 12 || resp.LastSortID == nil || *resp.LastSortID != "sort_123" {
		t.Fatalf("pagination = %#v, want has_more total_count last_sort_id", resp)
	}
}

func TestVoicesListSharedWithResponseReturnsRawMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req_voices")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"voices":[],"has_more":false}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Voices.ListSharedWithResponse(ctx, ListSharedVoicesRequest{})
	if err != nil {
		t.Fatalf("ListSharedWithResponse returned error: %v", err)
	}
	if len(resp.Data.Voices) != 0 || resp.Data.HasMore {
		t.Fatalf("Data = %#v, want empty page", resp.Data)
	}
	if resp.RawResponse.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.RawResponse.StatusCode, http.StatusOK)
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_voices" {
		t.Fatalf("X-Request-ID = %q, want req_voices", resp.RawResponse.Header.Get("X-Request-ID"))
	}
	if resp.RawResponse.URL != server.URL+"/v1/shared-voices" {
		t.Fatalf("URL = %q, want %s/v1/shared-voices", resp.RawResponse.URL, server.URL)
	}
}

func TestVoicesAddSharedSendsJSONAndParsesResponse(t *testing.T) {
	ctx := context.Background()
	bookmarked := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.EscapedPath() != "/v1/voices/add/public%2Fuser/voice%2Fid" {
			t.Fatalf("path = %s, want escaped add shared voice path", r.URL.EscapedPath())
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body AddSharedVoiceRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.NewName != "John Smith" {
			t.Fatalf("NewName = %q, want John Smith", body.NewName)
		}
		if body.Bookmarked == nil || *body.Bookmarked {
			t.Fatalf("Bookmarked = %#v, want false", body.Bookmarked)
		}

		w.Header().Set("X-Request-ID", "req_add_shared")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"voice_id":"b38kUX8pkfYO2kHyqfFy"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Voices.AddSharedWithResponse(ctx, "public/user", "voice/id", AddSharedVoiceRequest{
		Bookmarked: &bookmarked,
		NewName:    "John Smith",
	})
	if err != nil {
		t.Fatalf("AddSharedWithResponse returned error: %v", err)
	}
	if resp.Data.VoiceID != "b38kUX8pkfYO2kHyqfFy" {
		t.Fatalf("VoiceID = %q, want b38kUX8pkfYO2kHyqfFy", resp.Data.VoiceID)
	}
	if resp.RawResponse.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.RawResponse.StatusCode, http.StatusOK)
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_add_shared" {
		t.Fatalf("X-Request-ID = %q, want req_add_shared", resp.RawResponse.Header.Get("X-Request-ID"))
	}
}

func TestVoicesAddSharedOmitsNilBookmarked(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := body["bookmarked"]; ok {
			t.Fatalf("bookmarked present in body %#v, want omitted", body)
		}
		if body["new_name"] != "John Smith" {
			t.Fatalf("new_name = %q, want John Smith", body["new_name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"voice_id":"b38kUX8pkfYO2kHyqfFy"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Voices.AddShared(ctx, "public_user_id", "voice_id", AddSharedVoiceRequest{
		NewName: "John Smith",
	})
	if err != nil {
		t.Fatalf("AddShared returned error: %v", err)
	}
	if resp.VoiceID != "b38kUX8pkfYO2kHyqfFy" {
		t.Fatalf("VoiceID = %q, want b38kUX8pkfYO2kHyqfFy", resp.VoiceID)
	}
}

func TestVoicesAddSharedValidatesRequiredFields(t *testing.T) {
	ctx := context.Background()
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"voice_id":"unexpected"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	tests := []struct {
		name         string
		publicUserID string
		voiceID      string
		in           AddSharedVoiceRequest
		want         string
	}{
		{
			name:         "public user id",
			publicUserID: " ",
			voiceID:      "voice_id",
			in:           AddSharedVoiceRequest{NewName: "John Smith"},
			want:         "public_user_id is required",
		},
		{
			name:         "voice id",
			publicUserID: "public_user_id",
			voiceID:      " ",
			in:           AddSharedVoiceRequest{NewName: "John Smith"},
			want:         "voice_id is required",
		},
		{
			name:         "new name",
			publicUserID: "public_user_id",
			voiceID:      "voice_id",
			in:           AddSharedVoiceRequest{NewName: " "},
			want:         "new_name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Voices.AddShared(ctx, tt.publicUserID, tt.voiceID, tt.in)
			if err == nil {
				t.Fatal("AddShared returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}

	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

func TestVoicesCreatePVCSendsJSONAndParsesResponse(t *testing.T) {
	ctx := context.Background()
	description := "Warm narration voice"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/voices/pvc" {
			t.Fatalf("path = %s, want /v1/voices/pvc", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body CreatePVCVoiceRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Description == nil || *body.Description != "Warm narration voice" {
			t.Fatalf("Description = %#v, want Warm narration voice", body.Description)
		}
		if body.Labels["accent"] != "american" || body.Labels["gender"] != "male" {
			t.Fatalf("Labels = %#v, want accent and gender labels", body.Labels)
		}
		if body.Language != "en" {
			t.Fatalf("Language = %q, want en", body.Language)
		}
		if body.Name != "John Smith" {
			t.Fatalf("Name = %q, want John Smith", body.Name)
		}

		w.Header().Set("X-Request-ID", "req_create_pvc")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"voice_id":"b38kUX8pkfYO2kHyqfFy"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Voices.CreatePVCWithResponse(ctx, CreatePVCVoiceRequest{
		Description: &description,
		Labels: map[string]string{
			"accent": "american",
			"gender": "male",
		},
		Language: "en",
		Name:     "John Smith",
	})
	if err != nil {
		t.Fatalf("CreatePVCWithResponse returned error: %v", err)
	}
	if resp.Data.VoiceID != "b38kUX8pkfYO2kHyqfFy" {
		t.Fatalf("VoiceID = %q, want b38kUX8pkfYO2kHyqfFy", resp.Data.VoiceID)
	}
	if resp.RawResponse.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.RawResponse.StatusCode, http.StatusOK)
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_create_pvc" {
		t.Fatalf("X-Request-ID = %q, want req_create_pvc", resp.RawResponse.Header.Get("X-Request-ID"))
	}
}

func TestVoicesCreatePVCOmitsOptionalFields(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := body["description"]; ok {
			t.Fatalf("description present in body %#v, want omitted", body)
		}
		if _, ok := body["labels"]; ok {
			t.Fatalf("labels present in body %#v, want omitted", body)
		}
		if body["language"] != "en" {
			t.Fatalf("language = %q, want en", body["language"])
		}
		if body["name"] != "John Smith" {
			t.Fatalf("name = %q, want John Smith", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"voice_id":"b38kUX8pkfYO2kHyqfFy"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Voices.CreatePVC(ctx, CreatePVCVoiceRequest{
		Language: "en",
		Name:     "John Smith",
	})
	if err != nil {
		t.Fatalf("CreatePVC returned error: %v", err)
	}
	if resp.VoiceID != "b38kUX8pkfYO2kHyqfFy" {
		t.Fatalf("VoiceID = %q, want b38kUX8pkfYO2kHyqfFy", resp.VoiceID)
	}
}

func TestVoicesCreatePVCValidatesDocumentedFields(t *testing.T) {
	ctx := context.Background()
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"voice_id":"unexpected"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	tooLongDescription := strings.Repeat("d", 501)

	tests := []struct {
		in   CreatePVCVoiceRequest
		name string
		want string
	}{
		{
			in:   CreatePVCVoiceRequest{Language: "en"},
			name: "name required",
			want: "name is required",
		},
		{
			in: CreatePVCVoiceRequest{
				Language: "en",
				Name:     " ",
			},
			name: "name cannot be blank",
			want: "name is required",
		},
		{
			in: CreatePVCVoiceRequest{
				Language: "en",
				Name:     strings.Repeat("n", 101),
			},
			name: "name max length",
			want: "name must be 100 characters or fewer",
		},
		{
			in:   CreatePVCVoiceRequest{Name: "John Smith"},
			name: "language required",
			want: "language is required",
		},
		{
			in: CreatePVCVoiceRequest{
				Language: " ",
				Name:     "John Smith",
			},
			name: "language cannot be blank",
			want: "language is required",
		},
		{
			in: CreatePVCVoiceRequest{
				Description: &tooLongDescription,
				Language:    "en",
				Name:        "John Smith",
			},
			name: "description max length",
			want: "description must be 500 characters or fewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Voices.CreatePVC(ctx, tt.in)
			if err == nil {
				t.Fatal("CreatePVC returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}

	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

func TestSharedVoicesPathOmitsEmptyQuery(t *testing.T) {
	if got := sharedVoicesPath(ListSharedVoicesRequest{}); got != "/v1/shared-voices" {
		t.Fatalf("path = %q, want /v1/shared-voices", got)
	}
}

func assertVoiceQueryValues(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("query values = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("query values = %v, want %v", got, want)
		}
	}
}
