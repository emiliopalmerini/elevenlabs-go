package elevenlabs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// File is a file upload for multipart API requests.
type File struct {
	Name   string
	Reader io.Reader
}

// CreateTranscriptRequest contains parameters for creating a speech-to-text
// transcript.
type CreateTranscriptRequest struct {
	ModelID string

	File            *File
	SourceURL       string
	CloudStorageURL string

	LanguageCode            string
	TimestampsGranularity   string
	Diarize                 *bool
	Webhook                 *bool
	UseMultiChannel         *bool
	MultichannelOutputStyle string
	Keyterms                []string
	ExtraFormFields         map[string][]string
}

// Transcript is a speech-to-text transcript response.
type Transcript struct {
	Text                string           `json:"text"`
	LanguageCode        string           `json:"language_code,omitempty"`
	LanguageProbability float64          `json:"language_probability,omitempty"`
	Words               []TranscriptWord `json:"words,omitempty"`
}

// TranscriptWord is a word-level transcript segment.
type TranscriptWord struct {
	Text      string  `json:"text"`
	Type      string  `json:"type,omitempty"`
	Start     float64 `json:"start,omitempty"`
	End       float64 `json:"end,omitempty"`
	SpeakerID string  `json:"speaker_id,omitempty"`
}

// CreateTranscript transcribes an audio or video file.
func (c *Client) CreateTranscript(ctx context.Context, in CreateTranscriptRequest) (*Transcript, error) {
	if err := validateCreateTranscriptRequest(in); err != nil {
		return nil, err
	}

	body := createTranscriptBody(in)
	build := func(ctx context.Context) (*http.Request, error) {
		reader, err := body.newReader()
		if err != nil {
			return nil, err
		}

		req, err := c.newRequest(ctx, http.MethodPost, "/v1/speech-to-text", reader)
		if err != nil {
			if closer, ok := reader.(io.Closer); ok {
				_ = closer.Close()
			}
			return nil, err
		}
		req.Header.Set("Content-Type", body.contentType)

		return req, nil
	}

	var out Transcript
	if err := c.do(ctx, build, body.retryable, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetTranscript retrieves a previously generated transcript by ID.
func (c *Client) GetTranscript(ctx context.Context, id string) (*Transcript, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("elevenlabs: transcript id is required")
	}

	build := func(ctx context.Context) (*http.Request, error) {
		return c.newRequest(ctx, http.MethodGet, transcriptPath(id), nil)
	}

	var out Transcript
	if err := c.do(ctx, build, true, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// DeleteTranscript deletes a previously generated transcript by ID.
func (c *Client) DeleteTranscript(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("elevenlabs: transcript id is required")
	}

	build := func(ctx context.Context) (*http.Request, error) {
		return c.newRequest(ctx, http.MethodDelete, transcriptPath(id), nil)
	}

	return c.do(ctx, build, true, nil)
}

func transcriptPath(id string) string {
	return "/v1/speech-to-text/transcripts/" + url.PathEscape(id)
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

type transcriptBody struct {
	newReader   func() (io.Reader, error)
	contentType string
	retryable   bool
}

func createTranscriptBody(in CreateTranscriptRequest) transcriptBody {
	writer := multipart.NewWriter(io.Discard)
	boundary := writer.Boundary()
	contentType := writer.FormDataContentType()

	if in.File == nil {
		return transcriptBody{
			newReader: func() (io.Reader, error) {
				return createTranscriptBufferedBody(in, boundary)
			},
			contentType: contentType,
			retryable:   true,
		}
	}

	if seeker, ok := in.File.Reader.(io.ReadSeeker); ok {
		return transcriptBody{
			newReader: func() (io.Reader, error) {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return nil, fmt.Errorf("seek file: %w", err)
				}
				return createTranscriptStreamingBody(in, boundary)
			},
			contentType: contentType,
			retryable:   true,
		}
	}

	used := false
	return transcriptBody{
		newReader: func() (io.Reader, error) {
			if used {
				return nil, errors.New("elevenlabs: transcript file reader is not replayable")
			}
			used = true
			return createTranscriptStreamingBody(in, boundary)
		},
		contentType: contentType,
		retryable:   false,
	}
}

func createTranscriptBufferedBody(in CreateTranscriptRequest, boundary string) (io.Reader, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.SetBoundary(boundary); err != nil {
		return nil, err
	}
	if err := writeCreateTranscriptForm(mw, in); err != nil {
		_ = mw.Close()
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func createTranscriptStreamingBody(in CreateTranscriptRequest, boundary string) (io.Reader, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	if err := mw.SetBoundary(boundary); err != nil {
		_ = pr.Close()
		_ = pw.CloseWithError(err)
		return nil, err
	}

	go func() {
		err := writeCreateTranscriptForm(mw, in)
		closeErr := mw.Close()
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if closeErr != nil {
			_ = pw.CloseWithError(closeErr)
			return
		}
		_ = pw.Close()
	}()

	return pr, nil
}

func writeCreateTranscriptForm(mw *multipart.Writer, in CreateTranscriptRequest) error {
	if err := mw.WriteField("model_id", in.ModelID); err != nil {
		return err
	}
	if in.LanguageCode != "" {
		if err := mw.WriteField("language_code", in.LanguageCode); err != nil {
			return err
		}
	}
	if in.TimestampsGranularity != "" {
		if err := mw.WriteField("timestamps_granularity", in.TimestampsGranularity); err != nil {
			return err
		}
	}
	if in.Diarize != nil {
		if err := mw.WriteField("diarize", strconv.FormatBool(*in.Diarize)); err != nil {
			return err
		}
	}
	if in.Webhook != nil {
		if err := mw.WriteField("webhook", strconv.FormatBool(*in.Webhook)); err != nil {
			return err
		}
	}
	if in.UseMultiChannel != nil {
		if err := mw.WriteField("use_multi_channel", strconv.FormatBool(*in.UseMultiChannel)); err != nil {
			return err
		}
	}
	if in.MultichannelOutputStyle != "" {
		if err := mw.WriteField("multichannel_output_style", in.MultichannelOutputStyle); err != nil {
			return err
		}
	}
	for _, keyterm := range in.Keyterms {
		if err := mw.WriteField("keyterms", keyterm); err != nil {
			return err
		}
	}
	if in.SourceURL != "" {
		if err := mw.WriteField("source_url", in.SourceURL); err != nil {
			return err
		}
	}
	if in.CloudStorageURL != "" {
		if err := mw.WriteField("cloud_storage_url", in.CloudStorageURL); err != nil {
			return err
		}
	}
	if in.File != nil {
		part, err := mw.CreateFormFile("file", in.File.Name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(part, in.File.Reader); err != nil {
			return fmt.Errorf("copy file: %w", err)
		}
	}

	keys := make([]string, 0, len(in.ExtraFormFields))
	for key := range in.ExtraFormFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		for _, value := range in.ExtraFormFields[key] {
			if err := mw.WriteField(key, value); err != nil {
				return err
			}
		}
	}

	return nil
}
