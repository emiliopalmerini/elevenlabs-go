package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// CreateCompositionPlan creates a music composition plan from a prompt.
func (s *MusicService) CreateCompositionPlan(ctx context.Context, in CreateCompositionPlanRequest) (MusicCompositionPlan, error) {
	resp, err := s.CreateCompositionPlanWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateCompositionPlanWithResponse creates a music composition plan from a
// prompt and returns HTTP response metadata.
func (s *MusicService) CreateCompositionPlanWithResponse(ctx context.Context, in CreateCompositionPlanRequest) (*Response[MusicCompositionPlan], error) {
	body, raw, err := s.doCreateCompositionPlan(ctx, in)
	if err != nil {
		return nil, err
	}
	plan, err := decodeMusicCompositionPlan(body)
	if err != nil {
		return nil, err
	}
	return &Response[MusicCompositionPlan]{
		Data:        plan,
		RawResponse: raw,
	}, nil
}

func (s *MusicService) doCreateCompositionPlan(ctx context.Context, in CreateCompositionPlanRequest) ([]byte, RawResponse, error) {
	core, payload, err := s.prepareCreateCompositionPlanRequest(in)
	if err != nil {
		return nil, RawResponse{}, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, createCompositionPlanPath(), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	return core.Do(ctx, build, true)
}

func (s *MusicService) prepareCreateCompositionPlanRequest(in CreateCompositionPlanRequest) (*Client, []byte, error) {
	if err := validateCreateCompositionPlanRequest(in); err != nil {
		return nil, nil, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("elevenlabs: encode composition plan request: %w", err)
	}
	return core, payload, nil
}

func validateCreateCompositionPlanRequest(in CreateCompositionPlanRequest) error {
	if strings.TrimSpace(in.Prompt) == "" {
		return errors.New("elevenlabs: prompt is required")
	}
	if in.MusicLengthMS != nil && (*in.MusicLengthMS < 3000 || *in.MusicLengthMS > 600000) {
		return errors.New("elevenlabs: music_length_ms must be between 3000 and 600000")
	}
	return nil
}

func createCompositionPlanPath() string {
	return "/v1/music/plan"
}
