package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// ComposeDetailed generates music and returns audio with generated metadata.
func (s *MusicService) ComposeDetailed(ctx context.Context, in ComposeDetailedMusicRequest) (*DetailedMusicComposition, error) {
	resp, err := s.ComposeDetailedWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ComposeDetailedWithResponse generates music, returns audio with generated
// metadata, and includes HTTP response metadata.
func (s *MusicService) ComposeDetailedWithResponse(ctx context.Context, in ComposeDetailedMusicRequest) (*Response[*DetailedMusicComposition], error) {
	body, raw, err := s.doComposeDetailed(ctx, in)
	if err != nil {
		return nil, err
	}
	out, err := parseDetailedMusicComposition(body, raw)
	if err != nil {
		return nil, err
	}
	return &Response[*DetailedMusicComposition]{
		Data:        out,
		RawResponse: raw,
	}, nil
}

func (s *MusicService) doComposeDetailed(ctx context.Context, in ComposeDetailedMusicRequest) ([]byte, RawResponse, error) {
	core, payload, err := s.prepareComposeDetailedRequest(in)
	if err != nil {
		return nil, RawResponse{}, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, composeDetailedMusicPath(in), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	return core.Do(ctx, build, true)
}

func (s *MusicService) prepareComposeDetailedRequest(in ComposeDetailedMusicRequest) (*Client, []byte, error) {
	if err := validateComposeDetailedMusicRequest(in); err != nil {
		return nil, nil, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("elevenlabs: encode detailed music compose request: %w", err)
	}
	return core, payload, nil
}

func validateComposeDetailedMusicRequest(in ComposeDetailedMusicRequest) error {
	return validateComposeMusicRequest(ComposeMusicRequest{
		MusicLengthMS: in.MusicLengthMS,
		Prompt:        in.Prompt,
		Seed:          in.Seed,
	})
}

func composeDetailedMusicPath(in ComposeDetailedMusicRequest) string {
	path := "/v1/music/detailed"

	values := url.Values{}
	setStringQuery(values, "output_format", in.OutputFormat)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

func parseDetailedMusicComposition(body []byte, raw RawResponse) (*DetailedMusicComposition, error) {
	contentType := raw.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: parse detailed music content type: %w", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		return nil, fmt.Errorf("elevenlabs: expected multipart detailed music response, got %q", mediaType)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("elevenlabs: detailed music response missing multipart boundary")
	}

	out := &DetailedMusicComposition{
		SongID: raw.Header.Get("song-id"),
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("elevenlabs: read detailed music part: %w", err)
		}

		data, err := io.ReadAll(part)
		if err != nil {
			return nil, fmt.Errorf("elevenlabs: read detailed music part body: %w", err)
		}

		partMediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if partMediaType == "application/json" {
			if err := decodeDetailedMusicMetadata(data, out); err != nil {
				return nil, err
			}
			continue
		}
		out.Audio = append(out.Audio, data...)
	}

	if out.Audio == nil {
		return nil, fmt.Errorf("elevenlabs: detailed music response missing audio part")
	}
	return out, nil
}

type detailedMusicMetadata struct {
	CompositionPlan json.RawMessage    `json:"composition_plan"`
	SongMetadata    *MusicSongMetadata `json:"song_metadata"`
}

func decodeDetailedMusicMetadata(body []byte, out *DetailedMusicComposition) error {
	var metadata detailedMusicMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return fmt.Errorf("elevenlabs: decode detailed music metadata: %w", err)
	}
	if len(metadata.CompositionPlan) > 0 && string(metadata.CompositionPlan) != "null" {
		plan, err := decodeMusicCompositionPlan(metadata.CompositionPlan)
		if err != nil {
			return err
		}
		out.CompositionPlan = plan
	}
	out.SongMetadata = metadata.SongMetadata
	return nil
}

func decodeMusicCompositionPlan(body []byte) (MusicCompositionPlan, error) {
	var probe struct {
		Chunks   json.RawMessage `json:"chunks"`
		Sections json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("elevenlabs: decode music composition plan: %w", err)
	}
	if len(probe.Chunks) > 0 {
		var plan CompositionPlan
		if err := json.Unmarshal(body, &plan); err != nil {
			return nil, fmt.Errorf("elevenlabs: decode music composition plan: %w", err)
		}
		return plan, nil
	}
	if len(probe.Sections) > 0 {
		var prompt MusicPrompt
		if err := json.Unmarshal(body, &prompt); err != nil {
			return nil, fmt.Errorf("elevenlabs: decode music prompt: %w", err)
		}
		return prompt, nil
	}
	return nil, fmt.Errorf("elevenlabs: unknown music composition plan shape")
}

func (p *CompositionPlan) UnmarshalJSON(body []byte) error {
	var raw struct {
		Chunks []json.RawMessage `json:"chunks"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}
	p.Chunks = make([]CompositionPlanChunk, 0, len(raw.Chunks))
	for _, chunkBody := range raw.Chunks {
		chunk, err := decodeCompositionPlanChunk(chunkBody)
		if err != nil {
			return err
		}
		p.Chunks = append(p.Chunks, chunk)
	}
	return nil
}

func decodeCompositionPlanChunk(body []byte) (CompositionPlanChunk, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("elevenlabs: decode music composition chunk: %w", err)
	}
	if _, ok := probe["text"]; ok {
		var chunk GenerationChunkInput
		if err := json.Unmarshal(body, &chunk); err != nil {
			return nil, fmt.Errorf("elevenlabs: decode generated music chunk: %w", err)
		}
		return chunk, nil
	}
	var chunk AudioRefChunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		return nil, fmt.Errorf("elevenlabs: decode audio reference chunk: %w", err)
	}
	return chunk, nil
}
