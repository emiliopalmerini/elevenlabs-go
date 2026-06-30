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
)

// SpeechEngineService provides Speech Engine resource management APIs.
type SpeechEngineService struct {
	client *Client
}

// SpeechEngineListRequest contains filters and pagination settings for listing
// Speech Engine resources.
type SpeechEngineListRequest struct {
	PageSize      *int
	Search        string
	SortDirection string
	SortBy        string
	Cursor        string
}

// List returns a page of Speech Engine resources.
func (s *SpeechEngineService) List(ctx context.Context, in SpeechEngineListRequest) (*ListSpeechEnginesResponse, error) {
	resp, err := s.ListWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ListWithResponse returns a page of Speech Engine resources with HTTP response
// metadata.
func (s *SpeechEngineService) ListWithResponse(ctx context.Context, in SpeechEngineListRequest) (*Response[*ListSpeechEnginesResponse], error) {
	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}

	var out ListSpeechEnginesResponse
	raw, err := core.GetJSON(ctx, speechEngineListPath(in), &out)
	if err != nil {
		return nil, err
	}

	return &Response[*ListSpeechEnginesResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// Create creates a Speech Engine resource.
func (s *SpeechEngineService) Create(ctx context.Context, in SpeechEngineCreateRequest) (*SpeechEngineResponse, error) {
	resp, err := s.CreateWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateWithResponse creates a Speech Engine resource and returns HTTP response
// metadata.
func (s *SpeechEngineService) CreateWithResponse(ctx context.Context, in SpeechEngineCreateRequest) (*Response[*SpeechEngineResponse], error) {
	if err := validateSpeechEngineConfig(in.SpeechEngine); err != nil {
		return nil, err
	}

	var out SpeechEngineResponse
	raw, err := s.doJSON(ctx, http.MethodPost, "/v1/speech-engine", in, &out)
	if err != nil {
		return nil, err
	}

	return &Response[*SpeechEngineResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// Get retrieves a Speech Engine resource by ID.
func (s *SpeechEngineService) Get(ctx context.Context, id string) (*SpeechEngineResponse, error) {
	resp, err := s.GetWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetWithResponse retrieves a Speech Engine resource by ID and returns HTTP
// response metadata.
func (s *SpeechEngineService) GetWithResponse(ctx context.Context, id string) (*Response[*SpeechEngineResponse], error) {
	path, err := speechEngineIDPath(id)
	if err != nil {
		return nil, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}

	var out SpeechEngineResponse
	raw, err := core.GetJSON(ctx, path, &out)
	if err != nil {
		return nil, err
	}

	return &Response[*SpeechEngineResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// Update partially updates a Speech Engine resource.
func (s *SpeechEngineService) Update(ctx context.Context, id string, in SpeechEngineUpdateRequest) (*SpeechEngineResponse, error) {
	resp, err := s.UpdateWithResponse(ctx, id, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// UpdateWithResponse partially updates a Speech Engine resource and returns
// HTTP response metadata.
func (s *SpeechEngineService) UpdateWithResponse(ctx context.Context, id string, in SpeechEngineUpdateRequest) (*Response[*SpeechEngineResponse], error) {
	path, err := speechEngineIDPath(id)
	if err != nil {
		return nil, err
	}
	if in.SpeechEngine != nil {
		if err := validateSpeechEngineConfig(*in.SpeechEngine); err != nil {
			return nil, err
		}
	}

	var out SpeechEngineResponse
	raw, err := s.doJSON(ctx, http.MethodPatch, path, in, &out)
	if err != nil {
		return nil, err
	}

	return &Response[*SpeechEngineResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// Delete deletes a Speech Engine resource by ID.
func (s *SpeechEngineService) Delete(ctx context.Context, id string) error {
	_, err := s.DeleteWithResponse(ctx, id)
	return err
}

// DeleteWithResponse deletes a Speech Engine resource by ID and returns HTTP
// response metadata.
func (s *SpeechEngineService) DeleteWithResponse(ctx context.Context, id string) (*Response[any], error) {
	path, err := speechEngineIDPath(id)
	if err != nil {
		return nil, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		return core.NewRequest(ctx, http.MethodDelete, path, nil)
	}

	body, raw, err := core.Do(ctx, build, true)
	if err != nil {
		return nil, err
	}
	data, err := DecodeOptionalResponse(body)
	if err != nil {
		return nil, err
	}

	return &Response[any]{
		Data:        data,
		RawResponse: raw,
	}, nil
}

func (s *SpeechEngineService) doJSON(ctx context.Context, method, path string, in any, out any) (RawResponse, error) {
	core, err := s.apiClient()
	if err != nil {
		return RawResponse{}, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return RawResponse{}, fmt.Errorf("elevenlabs: encode speech engine request: %w", err)
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, method, path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	body, raw, err := core.Do(ctx, build, true)
	if err != nil {
		return raw, err
	}
	if err := DecodeResponse(body, out); err != nil {
		return raw, err
	}
	return raw, nil
}

func (s *SpeechEngineService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}

func speechEngineListPath(in SpeechEngineListRequest) string {
	path := "/v1/speech-engine"
	values := url.Values{}
	setIntQuery(values, "page_size", in.PageSize)
	setStringQuery(values, "search", in.Search)
	setStringQuery(values, "sort_direction", in.SortDirection)
	setStringQuery(values, "sort_by", in.SortBy)
	setStringQuery(values, "cursor", in.Cursor)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

func speechEngineIDPath(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("elevenlabs: speech engine id is required")
	}
	return "/v1/speech-engine/" + url.PathEscape(id), nil
}

func validateSpeechEngineConfig(in SpeechEngineConfig) error {
	if strings.TrimSpace(in.WSURL) == "" {
		return errors.New("elevenlabs: ws_url is required")
	}
	return nil
}
