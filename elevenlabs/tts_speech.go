package elevenlabs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// CreateSpeech converts text into speech and returns the generated audio bytes.
func (c *TTSService) CreateSpeech(ctx context.Context, in CreateSpeechRequest) ([]byte, error) {
	resp, err := c.CreateSpeechWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateSpeechWithResponse converts text into speech and returns HTTP response
// metadata.
func (c *TTSService) CreateSpeechWithResponse(ctx context.Context, in CreateSpeechRequest) (*Response[[]byte], error) {
	body, raw, err := c.doSpeech(ctx, in, "")
	if err != nil {
		return nil, err
	}
	return &Response[[]byte]{
		Data:        body,
		RawResponse: raw,
	}, nil
}

// CreateSpeechWithTimestamps converts text into speech and returns generated
// audio with character-level timing information.
func (c *TTSService) CreateSpeechWithTimestamps(ctx context.Context, in CreateSpeechRequest) (*AudioWithTimestamps, error) {
	resp, err := c.CreateSpeechWithTimestampsWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateSpeechWithTimestampsWithResponse converts text into speech with timing
// information and returns HTTP response metadata.
func (c *TTSService) CreateSpeechWithTimestampsWithResponse(ctx context.Context, in CreateSpeechRequest) (*Response[*AudioWithTimestamps], error) {
	var out AudioWithTimestamps
	raw, err := c.doSpeechJSON(ctx, in, "/with-timestamps", &out)
	if err != nil {
		return nil, err
	}
	return &Response[*AudioWithTimestamps]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// StreamSpeech converts text into speech and returns a streaming audio body.
// The caller must close the returned stream.
func (c *TTSService) StreamSpeech(ctx context.Context, in CreateSpeechRequest) (*AudioStream, error) {
	resp, err := c.StreamSpeechWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// StreamSpeechWithResponse converts text into speech, returns a streaming audio
// body, and includes HTTP response metadata. The caller must close Data.
func (c *TTSService) StreamSpeechWithResponse(ctx context.Context, in CreateSpeechRequest) (*Response[*AudioStream], error) {
	body, raw, err := c.doSpeechStream(ctx, in, "/stream")
	if err != nil {
		return nil, err
	}
	return &Response[*AudioStream]{
		Data:        newAudioStream(body),
		RawResponse: raw,
	}, nil
}

// StreamSpeechWithTimestamps converts text into speech and returns a stream of
// timestamped audio chunks. The caller must close the returned stream.
func (c *TTSService) StreamSpeechWithTimestamps(ctx context.Context, in CreateSpeechRequest) (*TimestampStream, error) {
	resp, err := c.StreamSpeechWithTimestampsWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// StreamSpeechWithTimestampsWithResponse converts text into speech, returns a
// stream of timestamped audio chunks, and includes HTTP response metadata. The
// caller must close Data.
func (c *TTSService) StreamSpeechWithTimestampsWithResponse(ctx context.Context, in CreateSpeechRequest) (*Response[*TimestampStream], error) {
	body, raw, err := c.doSpeechStream(ctx, in, "/stream/with-timestamps")
	if err != nil {
		return nil, err
	}
	return &Response[*TimestampStream]{
		Data:        newTimestampStream(body),
		RawResponse: raw,
	}, nil
}

func (c *TTSService) doSpeech(ctx context.Context, in CreateSpeechRequest, suffix string) ([]byte, RawResponse, error) {
	core, payload, err := c.prepareSpeechRequest(in)
	if err != nil {
		return nil, RawResponse{}, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, speechPath(in, suffix), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	return core.Do(ctx, build, true)
}

func (c *TTSService) doSpeechJSON(ctx context.Context, in CreateSpeechRequest, suffix string, out any) (RawResponse, error) {
	body, raw, err := c.doSpeech(ctx, in, suffix)
	if err != nil {
		return raw, err
	}
	if err := DecodeResponse(body, out); err != nil {
		return raw, err
	}
	return raw, nil
}

func (c *TTSService) doSpeechStream(ctx context.Context, in CreateSpeechRequest, suffix string) (io.ReadCloser, RawResponse, error) {
	core, payload, err := c.prepareSpeechRequest(in)
	if err != nil {
		return nil, RawResponse{}, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		req, err := core.NewRequest(ctx, http.MethodPost, speechPath(in, suffix), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	return core.DoStream(ctx, build, true)
}

func (c *TTSService) prepareSpeechRequest(in CreateSpeechRequest) (*Client, []byte, error) {
	if err := validateCreateSpeechRequest(in); err != nil {
		return nil, nil, err
	}
	core, err := c.apiClient()
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("elevenlabs: encode speech request: %w", err)
	}
	return core, payload, nil
}

func validateCreateSpeechRequest(in CreateSpeechRequest) error {
	if strings.TrimSpace(in.VoiceID) == "" {
		return errors.New("elevenlabs: voice_id is required")
	}
	if strings.TrimSpace(in.Text) == "" {
		return errors.New("elevenlabs: text is required")
	}
	if in.OptimizeStreamingLatency != nil && (*in.OptimizeStreamingLatency < 0 || *in.OptimizeStreamingLatency > 4) {
		return errors.New("elevenlabs: optimize_streaming_latency must be between 0 and 4")
	}
	return nil
}

func speechPath(in CreateSpeechRequest, suffix string) string {
	path := "/v1/text-to-speech/" + url.PathEscape(strings.TrimSpace(in.VoiceID)) + suffix

	values := url.Values{}
	setBoolQuery(values, "enable_logging", in.EnableLogging)
	setIntQuery(values, "optimize_streaming_latency", in.OptimizeStreamingLatency)
	setStringQuery(values, "output_format", in.OutputFormat)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

// TimestampStream reads timestamped audio chunks from an HTTP streaming
// response.
type TimestampStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
}

func newTimestampStream(body io.ReadCloser) *TimestampStream {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	return &TimestampStream{body: body, scanner: scanner}
}

// Receive reads one timestamped audio chunk from the stream. It supports
// newline-delimited JSON chunks and simple text/event-stream data frames.
func (s *TimestampStream) Receive() (*AudioWithTimestamps, error) {
	if s == nil || s.body == nil || s.scanner == nil {
		return nil, errors.New("elevenlabs: nil timestamp stream")
	}

	var dataLines []string
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" {
			if len(dataLines) > 0 {
				return decodeTimestampChunk(strings.Join(dataLines, "\n"))
			}
			continue
		}
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return nil, io.EOF
			}
			dataLines = append(dataLines, data)
			continue
		}
		return decodeTimestampChunk(line)
	}

	if len(dataLines) > 0 {
		return decodeTimestampChunk(strings.Join(dataLines, "\n"))
	}
	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("elevenlabs: read timestamp stream: %w", err)
	}
	return nil, io.EOF
}

// Close closes the timestamp stream.
func (s *TimestampStream) Close() error {
	if s == nil || s.body == nil {
		return nil
	}
	return s.body.Close()
}

func decodeTimestampChunk(payload string) (*AudioWithTimestamps, error) {
	var chunk AudioWithTimestamps
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return nil, fmt.Errorf("elevenlabs: decode timestamp stream chunk: %w", err)
	}
	return &chunk, nil
}
