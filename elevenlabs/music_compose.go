package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"unicode/utf8"
)

// Compose generates music from a prompt or composition plan.
func (s *MusicService) Compose(ctx context.Context, in ComposeMusicRequest) (*MusicComposition, error) {
	resp, err := s.ComposeWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ComposeWithResponse generates music from a prompt or composition plan and
// returns HTTP response metadata.
func (s *MusicService) ComposeWithResponse(ctx context.Context, in ComposeMusicRequest) (*Response[*MusicComposition], error) {
	body, raw, err := s.doCompose(ctx, in)
	if err != nil {
		return nil, err
	}
	return &Response[*MusicComposition]{
		Data: &MusicComposition{
			Audio:  body,
			SongID: raw.Header.Get("song-id"),
		},
		RawResponse: raw,
	}, nil
}

func (s *MusicService) doCompose(ctx context.Context, in ComposeMusicRequest) ([]byte, RawResponse, error) {
	core, payload, err := s.prepareComposeRequest(in)
	if err != nil {
		return nil, RawResponse{}, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, composeMusicPath(in), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	return core.Do(ctx, build, true)
}

func (s *MusicService) prepareComposeRequest(in ComposeMusicRequest) (*Client, []byte, error) {
	if err := validateComposeMusicRequest(in); err != nil {
		return nil, nil, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("elevenlabs: encode music compose request: %w", err)
	}
	return core, payload, nil
}

func validateComposeMusicRequest(in ComposeMusicRequest) error {
	if utf8.RuneCountInString(in.Prompt) > 4100 {
		return errors.New("elevenlabs: prompt must be 4100 characters or fewer")
	}
	if in.MusicLengthMS != nil && (*in.MusicLengthMS < 3000 || *in.MusicLengthMS > 600000) {
		return errors.New("elevenlabs: music_length_ms must be between 3000 and 600000")
	}
	if in.Seed != nil && (*in.Seed < 0 || *in.Seed > 2147483647) {
		return errors.New("elevenlabs: seed must be between 0 and 2147483647")
	}
	return nil
}

func composeMusicPath(in ComposeMusicRequest) string {
	path := "/v1/music"

	values := url.Values{}
	setStringQuery(values, "output_format", in.OutputFormat)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
