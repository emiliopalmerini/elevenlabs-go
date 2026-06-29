package elevenlabs

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CreateTranscript transcribes an audio or video file.
func (c *STTService) CreateTranscript(ctx context.Context, in CreateTranscriptRequest) (*Transcript, error) {
	resp, err := c.CreateTranscriptWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateTranscriptWithResponse transcribes an audio or video file and returns
// HTTP response metadata.
func (c *STTService) CreateTranscriptWithResponse(ctx context.Context, in CreateTranscriptRequest) (*Response[*Transcript], error) {
	if err := validateCreateTranscriptRequest(in); err != nil {
		return nil, err
	}

	var out Transcript
	raw, err := c.doCreateTranscript(ctx, in, &out)
	if err != nil {
		return nil, err
	}

	return &Response[*Transcript]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// SubmitTranscriptWebhook submits a transcript request for asynchronous webhook
// processing.
func (c *STTService) SubmitTranscriptWebhook(ctx context.Context, in CreateTranscriptRequest) (*TranscriptWebhookResponse, error) {
	resp, err := c.SubmitTranscriptWebhookWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// SubmitTranscriptWebhookWithResponse submits a transcript request for
// asynchronous webhook processing and returns HTTP response metadata.
func (c *STTService) SubmitTranscriptWebhookWithResponse(ctx context.Context, in CreateTranscriptRequest) (*Response[*TranscriptWebhookResponse], error) {
	webhook := true
	in.Webhook = &webhook

	if err := validateCreateTranscriptRequest(in); err != nil {
		return nil, err
	}

	var out TranscriptWebhookResponse
	raw, err := c.doCreateTranscript(ctx, in, &out)
	if err != nil {
		return nil, err
	}

	return &Response[*TranscriptWebhookResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

func (c *STTService) doCreateTranscript(ctx context.Context, in CreateTranscriptRequest, out any) (RawResponse, error) {
	core, err := c.apiClient()
	if err != nil {
		return RawResponse{}, err
	}

	body := createTranscriptBody(in)
	attempt := 0
	build := func(ctx context.Context) (*http.Request, error) {
		attempt++
		reader, err := body.newReader(attempt)
		if err != nil {
			return nil, err
		}

		req, err := core.NewRequest(ctx, http.MethodPost, createTranscriptPath(in), reader)
		if err != nil {
			if closer, ok := reader.(io.Closer); ok {
				_ = closer.Close()
			}
			return nil, err
		}
		req.Header.Set("Content-Type", body.contentType)

		return req, nil
	}

	respBody, raw, err := core.Do(ctx, build, body.retryable)
	if err != nil {
		return raw, err
	}
	if err := DecodeResponse(respBody, out); err != nil {
		return raw, err
	}
	return raw, nil
}

// GetTranscript retrieves a previously generated transcript by ID.
func (c *STTService) GetTranscript(ctx context.Context, id string) (*Transcript, error) {
	resp, err := c.GetTranscriptWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetTranscriptWithResponse retrieves a previously generated transcript by ID
// and returns HTTP response metadata.
func (c *STTService) GetTranscriptWithResponse(ctx context.Context, id string) (*Response[*Transcript], error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("elevenlabs: transcript id is required")
	}
	core, err := c.apiClient()
	if err != nil {
		return nil, err
	}

	var out Transcript
	raw, err := core.GetJSON(ctx, transcriptPath(id), &out)
	if err != nil {
		return nil, err
	}

	return &Response[*Transcript]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// DeleteTranscript deletes a previously generated transcript by ID.
func (c *STTService) DeleteTranscript(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("elevenlabs: transcript id is required")
	}
	core, err := c.apiClient()
	if err != nil {
		return err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		return core.NewRequest(ctx, http.MethodDelete, transcriptPath(id), nil)
	}

	_, _, err = core.Do(ctx, build, true)
	return err
}

// DeleteTranscriptWithResponse deletes a previously generated transcript by ID
// and returns HTTP response metadata.
func (c *STTService) DeleteTranscriptWithResponse(ctx context.Context, id string) (*Response[any], error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("elevenlabs: transcript id is required")
	}
	core, err := c.apiClient()
	if err != nil {
		return nil, err
	}

	build := func(ctx context.Context) (*http.Request, error) {
		return core.NewRequest(ctx, http.MethodDelete, transcriptPath(id), nil)
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

func transcriptPath(id string) string {
	return "/v1/speech-to-text/transcripts/" + url.PathEscape(id)
}

func createTranscriptPath(in CreateTranscriptRequest) string {
	path := "/v1/speech-to-text"
	if in.EnableLogging == nil {
		return path
	}

	values := url.Values{}
	values.Set("enable_logging", strconv.FormatBool(*in.EnableLogging))
	return path + "?" + values.Encode()
}

func validateCreateTranscriptRequest(in CreateTranscriptRequest) error {
	if strings.TrimSpace(in.ModelID) == "" {
		return errors.New("elevenlabs: model_id is required")
	}

	sources := 0
	if in.File != nil {
		sources++
		if strings.TrimSpace(in.File.Name) == "" {
			return errors.New("elevenlabs: file name is required")
		}
		if in.File.Reader == nil {
			return errors.New("elevenlabs: file reader is required")
		}
	}
	if strings.TrimSpace(in.SourceURL) != "" {
		sources++
	}
	if strings.TrimSpace(in.CloudStorageURL) != "" {
		sources++
	}

	switch sources {
	case 0:
		return errors.New("elevenlabs: one audio source is required")
	case 1:
		return nil
	default:
		return errors.New("elevenlabs: only one audio source can be set")
	}
}
