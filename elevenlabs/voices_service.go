package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// VoicesService provides ElevenLabs voice APIs.
type VoicesService struct {
	client *Client
}

// ListShared retrieves a page of shared voices from the voice library.
func (s *VoicesService) ListShared(ctx context.Context, in ListSharedVoicesRequest) (*SharedVoicesResponse, error) {
	resp, err := s.ListSharedWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ListSharedWithResponse retrieves a page of shared voices from the voice
// library and returns HTTP response metadata.
func (s *VoicesService) ListSharedWithResponse(ctx context.Context, in ListSharedVoicesRequest) (*Response[*SharedVoicesResponse], error) {
	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}

	var out SharedVoicesResponse
	raw, err := core.GetJSON(ctx, sharedVoicesPath(in), &out)
	if err != nil {
		return nil, err
	}

	return &Response[*SharedVoicesResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// AddShared adds a shared voice to the authenticated user's voice collection.
func (s *VoicesService) AddShared(ctx context.Context, publicUserID, voiceID string, in AddSharedVoiceRequest) (*AddSharedVoiceResponse, error) {
	resp, err := s.AddSharedWithResponse(ctx, publicUserID, voiceID, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// AddSharedWithResponse adds a shared voice to the authenticated user's voice
// collection and returns HTTP response metadata.
func (s *VoicesService) AddSharedWithResponse(ctx context.Context, publicUserID, voiceID string, in AddSharedVoiceRequest) (*Response[*AddSharedVoiceResponse], error) {
	path, err := addSharedVoicePath(publicUserID, voiceID)
	if err != nil {
		return nil, err
	}
	if err := validateAddSharedVoiceRequest(in); err != nil {
		return nil, err
	}

	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: encode add shared voice request: %w", err)
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	body, raw, err := core.Do(ctx, build, true)
	if err != nil {
		return nil, err
	}

	var out AddSharedVoiceResponse
	if err := DecodeResponse(body, &out); err != nil {
		return nil, err
	}

	return &Response[*AddSharedVoiceResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// CreatePVC creates a PVC voice with metadata but no samples.
func (s *VoicesService) CreatePVC(ctx context.Context, in CreatePVCVoiceRequest) (*CreatePVCVoiceResponse, error) {
	resp, err := s.CreatePVCWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreatePVCWithResponse creates a PVC voice with metadata but no samples and
// returns HTTP response metadata.
func (s *VoicesService) CreatePVCWithResponse(ctx context.Context, in CreatePVCVoiceRequest) (*Response[*CreatePVCVoiceResponse], error) {
	if err := validateCreatePVCVoiceRequest(in); err != nil {
		return nil, err
	}

	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: encode create PVC voice request: %w", err)
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, createPVCVoicePath(), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	body, raw, err := core.Do(ctx, build, true)
	if err != nil {
		return nil, err
	}

	var out CreatePVCVoiceResponse
	if err := DecodeResponse(body, &out); err != nil {
		return nil, err
	}

	return &Response[*CreatePVCVoiceResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// UpdatePVC edits PVC voice metadata.
func (s *VoicesService) UpdatePVC(ctx context.Context, voiceID string, in UpdatePVCVoiceRequest) (*UpdatePVCVoiceResponse, error) {
	resp, err := s.UpdatePVCWithResponse(ctx, voiceID, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// UpdatePVCWithResponse edits PVC voice metadata and returns HTTP response
// metadata.
func (s *VoicesService) UpdatePVCWithResponse(ctx context.Context, voiceID string, in UpdatePVCVoiceRequest) (*Response[*UpdatePVCVoiceResponse], error) {
	path, err := updatePVCVoicePath(voiceID)
	if err != nil {
		return nil, err
	}
	if err := validateUpdatePVCVoiceRequest(in); err != nil {
		return nil, err
	}

	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: encode update PVC voice request: %w", err)
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	body, raw, err := core.Do(ctx, build, true)
	if err != nil {
		return nil, err
	}

	var out UpdatePVCVoiceResponse
	if err := DecodeResponse(body, &out); err != nil {
		return nil, err
	}

	return &Response[*UpdatePVCVoiceResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

func (s *VoicesService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}

func addSharedVoicePath(publicUserID, voiceID string) (string, error) {
	publicUserID = strings.TrimSpace(publicUserID)
	if publicUserID == "" {
		return "", errors.New("elevenlabs: public_user_id is required")
	}
	voiceID = strings.TrimSpace(voiceID)
	if voiceID == "" {
		return "", errors.New("elevenlabs: voice_id is required")
	}
	return "/v1/voices/add/" + url.PathEscape(publicUserID) + "/" + url.PathEscape(voiceID), nil
}

func validateAddSharedVoiceRequest(in AddSharedVoiceRequest) error {
	if strings.TrimSpace(in.NewName) == "" {
		return errors.New("elevenlabs: new_name is required")
	}
	return nil
}

func validateCreatePVCVoiceRequest(in CreatePVCVoiceRequest) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("elevenlabs: name is required")
	}
	if utf8.RuneCountInString(in.Name) > 100 {
		return errors.New("elevenlabs: name must be 100 characters or fewer")
	}
	if strings.TrimSpace(in.Language) == "" {
		return errors.New("elevenlabs: language is required")
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 500 {
		return errors.New("elevenlabs: description must be 500 characters or fewer")
	}
	return nil
}

func validateUpdatePVCVoiceRequest(in UpdatePVCVoiceRequest) error {
	if in.Name != "" && utf8.RuneCountInString(in.Name) > 100 {
		return errors.New("elevenlabs: name must be 100 characters or fewer")
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 500 {
		return errors.New("elevenlabs: description must be 500 characters or fewer")
	}
	return nil
}

func createPVCVoicePath() string {
	return "/v1/voices/pvc"
}

func updatePVCVoicePath(voiceID string) (string, error) {
	voiceID = strings.TrimSpace(voiceID)
	if voiceID == "" {
		return "", errors.New("elevenlabs: voice_id is required")
	}
	return "/v1/voices/pvc/" + url.PathEscape(voiceID), nil
}

func sharedVoicesPath(in ListSharedVoicesRequest) string {
	path := "/v1/shared-voices"
	values := url.Values{}
	setIntQuery(values, "page_size", in.PageSize)
	setStringQuery(values, "category", in.Category)
	setStringQuery(values, "gender", in.Gender)
	setStringQuery(values, "age", in.Age)
	setStringQuery(values, "accent", in.Accent)
	setStringQuery(values, "language", in.Language)
	setStringQuery(values, "locale", in.Locale)
	setStringQuery(values, "search", in.Search)
	addStringListQuery(values, "use_cases", in.UseCases)
	addStringListQuery(values, "descriptives", in.Descriptives)
	setBoolQuery(values, "featured", in.Featured)
	setIntQuery(values, "min_notice_period_days", in.MinNoticePeriodDays)
	setBoolQuery(values, "include_custom_rates", in.IncludeCustomRates)
	setBoolQuery(values, "include_live_moderated", in.IncludeLiveModerated)
	setBoolQuery(values, "reader_app_enabled", in.ReaderAppEnabled)
	setStringQuery(values, "owner_id", in.OwnerID)
	setStringQuery(values, "sort", in.Sort)
	setIntQuery(values, "page", in.Page)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

func addStringListQuery(query url.Values, name string, values []string) {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			query.Add(name, value)
		}
	}
}
