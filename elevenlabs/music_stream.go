package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Stream generates music from a prompt or composition plan and returns a
// streaming audio body. The caller must close the returned stream.
func (s *MusicService) Stream(ctx context.Context, in StreamMusicRequest) (*MusicStream, error) {
	resp, err := s.StreamWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// StreamWithResponse generates music from a prompt or composition plan, returns
// a streaming audio body, and includes HTTP response metadata. The caller must
// close Data.
func (s *MusicService) StreamWithResponse(ctx context.Context, in StreamMusicRequest) (*Response[*MusicStream], error) {
	body, raw, err := s.doStream(ctx, in)
	if err != nil {
		return nil, err
	}
	return &Response[*MusicStream]{
		Data: &MusicStream{
			Body:   body,
			SongID: raw.Header.Get("song-id"),
		},
		RawResponse: raw,
	}, nil
}

func (s *MusicService) doStream(ctx context.Context, in StreamMusicRequest) (io.ReadCloser, RawResponse, error) {
	core, payload, err := s.prepareStreamRequest(in)
	if err != nil {
		return nil, RawResponse{}, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, streamMusicPath(in), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	return core.DoStream(ctx, build, true)
}

func (s *MusicService) prepareStreamRequest(in StreamMusicRequest) (*Client, []byte, error) {
	if err := validateStreamMusicRequest(in); err != nil {
		return nil, nil, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("elevenlabs: encode music stream request: %w", err)
	}
	return core, payload, nil
}

func validateStreamMusicRequest(in StreamMusicRequest) error {
	return validateComposeMusicRequest(ComposeMusicRequest{
		MusicLengthMS: in.MusicLengthMS,
		Prompt:        in.Prompt,
		Seed:          in.Seed,
	})
}

func streamMusicPath(in StreamMusicRequest) string {
	path := "/v1/music/stream"

	values := url.Values{}
	setStringQuery(values, "output_format", in.OutputFormat)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
