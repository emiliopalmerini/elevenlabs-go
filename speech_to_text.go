package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
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
	EnableLogging   *bool

	LanguageCode            string
	TimestampsGranularity   string
	AdditionalFormats       []TranscriptAdditionalFormatOptions
	Diarize                 *bool
	DiarizationThreshold    *float64
	NumSpeakers             int
	TagAudioEvents          *bool
	NoVerbatim              *bool
	Webhook                 *bool
	WebhookID               string
	WebhookMetadata         map[string]any
	FileFormat              string
	Temperature             *float64
	Seed                    *int
	UseMultiChannel         *bool
	MultichannelOutputStyle string
	EntityDetection         []string
	UseSpeakerLibrary       *bool
	DetectSpeakerRoles      *bool
	EntityRedaction         []string
	EntityRedactionMode     string
	Keyterms                []string
	ExtraFormFields         map[string][]string
}

// Transcript is a speech-to-text transcript response.
type Transcript struct {
	Text                string                       `json:"text,omitempty"`
	LanguageCode        string                       `json:"language_code,omitempty"`
	LanguageProbability float64                      `json:"language_probability,omitempty"`
	Words               []TranscriptWord             `json:"words,omitempty"`
	ChannelIndex        *int                         `json:"channel_index,omitempty"`
	AdditionalFormats   []TranscriptAdditionalFormat `json:"additional_formats,omitempty"`
	TranscriptionID     string                       `json:"transcription_id,omitempty"`
	AudioDurationSecs   *float64                     `json:"audio_duration_secs,omitempty"`
	Entities            []DetectedEntity             `json:"entities,omitempty"`
	Transcripts         []Transcript                 `json:"transcripts,omitempty"`
}

// TranscriptWord is a word-level transcript segment.
type TranscriptWord struct {
	Text         string                `json:"text"`
	Type         string                `json:"type,omitempty"`
	Start        *float64              `json:"start,omitempty"`
	End          *float64              `json:"end,omitempty"`
	SpeakerID    string                `json:"speaker_id,omitempty"`
	Logprob      float64               `json:"logprob,omitempty"`
	Characters   []TranscriptCharacter `json:"characters,omitempty"`
	ChannelIndex *int                  `json:"channel_index,omitempty"`
}

// TranscriptCharacter is a character-level transcript segment.
type TranscriptCharacter struct {
	Text  string   `json:"text"`
	Start *float64 `json:"start,omitempty"`
	End   *float64 `json:"end,omitempty"`
}

// DetectedEntity is an entity detected in a transcript.
type DetectedEntity struct {
	Text       string `json:"text"`
	EntityType string `json:"entity_type"`
	StartChar  int    `json:"start_char"`
	EndChar    int    `json:"end_char"`
}

// TranscriptAdditionalFormatOptions configures an additional transcript export
// format.
type TranscriptAdditionalFormatOptions struct {
	Format                      string   `json:"format"`
	IncludeSpeakers             *bool    `json:"include_speakers,omitempty"`
	IncludeTimestamps           *bool    `json:"include_timestamps,omitempty"`
	SegmentOnSilenceLongerThanS *float64 `json:"segment_on_silence_longer_than_s,omitempty"`
	MaxSegmentDurationS         *float64 `json:"max_segment_duration_s,omitempty"`
	MaxSegmentChars             *int     `json:"max_segment_chars,omitempty"`
	MaxCharactersPerLine        *int     `json:"max_characters_per_line,omitempty"`
}

// TranscriptAdditionalFormat is an additional transcript export returned by
// the API.
type TranscriptAdditionalFormat struct {
	RequestedFormat string `json:"requested_format"`
	FileExtension   string `json:"file_extension"`
	ContentType     string `json:"content_type"`
	IsBase64Encoded bool   `json:"is_base64_encoded"`
	Content         string `json:"content"`
}

// TranscriptWebhookResponse is returned when a transcript is submitted for
// asynchronous webhook processing.
type TranscriptWebhookResponse struct {
	Message         string  `json:"message"`
	RequestID       string  `json:"request_id"`
	TranscriptionID *string `json:"transcription_id,omitempty"`
}

// CreateTranscript transcribes an audio or video file.
func (c *Client) CreateTranscript(ctx context.Context, in CreateTranscriptRequest) (*Transcript, error) {
	resp, err := c.CreateTranscriptWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateTranscriptWithResponse transcribes an audio or video file and returns
// HTTP response metadata.
func (c *Client) CreateTranscriptWithResponse(ctx context.Context, in CreateTranscriptRequest) (*Response[*Transcript], error) {
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
func (c *Client) SubmitTranscriptWebhook(ctx context.Context, in CreateTranscriptRequest) (*TranscriptWebhookResponse, error) {
	resp, err := c.SubmitTranscriptWebhookWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// SubmitTranscriptWebhookWithResponse submits a transcript request for
// asynchronous webhook processing and returns HTTP response metadata.
func (c *Client) SubmitTranscriptWebhookWithResponse(ctx context.Context, in CreateTranscriptRequest) (*Response[*TranscriptWebhookResponse], error) {
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

func (c *Client) doCreateTranscript(ctx context.Context, in CreateTranscriptRequest, out any) (RawResponse, error) {
	body := createTranscriptBody(in)
	build := func(ctx context.Context) (*http.Request, error) {
		reader, err := body.newReader()
		if err != nil {
			return nil, err
		}

		req, err := c.newRequest(ctx, http.MethodPost, createTranscriptPath(in), reader)
		if err != nil {
			if closer, ok := reader.(io.Closer); ok {
				_ = closer.Close()
			}
			return nil, err
		}
		req.Header.Set("Content-Type", body.contentType)

		return req, nil
	}

	respBody, raw, err := c.do(ctx, build, body.retryable)
	if err != nil {
		return raw, err
	}
	if err := decodeResponse(respBody, out); err != nil {
		return raw, err
	}
	return raw, nil
}

// GetTranscript retrieves a previously generated transcript by ID.
func (c *Client) GetTranscript(ctx context.Context, id string) (*Transcript, error) {
	resp, err := c.GetTranscriptWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetTranscriptWithResponse retrieves a previously generated transcript by ID
// and returns HTTP response metadata.
func (c *Client) GetTranscriptWithResponse(ctx context.Context, id string) (*Response[*Transcript], error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("elevenlabs: transcript id is required")
	}

	var out Transcript
	raw, err := c.getJSON(ctx, transcriptPath(id), &out)
	if err != nil {
		return nil, err
	}

	return &Response[*Transcript]{
		Data:        &out,
		RawResponse: raw,
	}, nil
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

	_, _, err := c.do(ctx, build, true)
	return err
}

// DeleteTranscriptWithResponse deletes a previously generated transcript by ID
// and returns HTTP response metadata.
func (c *Client) DeleteTranscriptWithResponse(ctx context.Context, id string) (*Response[any], error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("elevenlabs: transcript id is required")
	}

	build := func(ctx context.Context) (*http.Request, error) {
		return c.newRequest(ctx, http.MethodDelete, transcriptPath(id), nil)
	}

	body, raw, err := c.do(ctx, build, true)
	if err != nil {
		return nil, err
	}
	data, err := decodeOptionalResponse(body)
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
	if len(in.AdditionalFormats) > 0 {
		formats, err := json.Marshal(in.AdditionalFormats)
		if err != nil {
			return fmt.Errorf("marshal additional_formats: %w", err)
		}
		if err := mw.WriteField("additional_formats", string(formats)); err != nil {
			return err
		}
	}
	if in.Diarize != nil {
		if err := mw.WriteField("diarize", strconv.FormatBool(*in.Diarize)); err != nil {
			return err
		}
	}
	if in.DiarizationThreshold != nil {
		if err := mw.WriteField("diarization_threshold", strconv.FormatFloat(*in.DiarizationThreshold, 'f', -1, 64)); err != nil {
			return err
		}
	}
	if in.NumSpeakers > 0 {
		if err := mw.WriteField("num_speakers", strconv.Itoa(in.NumSpeakers)); err != nil {
			return err
		}
	}
	if in.TagAudioEvents != nil {
		if err := mw.WriteField("tag_audio_events", strconv.FormatBool(*in.TagAudioEvents)); err != nil {
			return err
		}
	}
	if in.NoVerbatim != nil {
		if err := mw.WriteField("no_verbatim", strconv.FormatBool(*in.NoVerbatim)); err != nil {
			return err
		}
	}
	if in.Webhook != nil {
		if err := mw.WriteField("webhook", strconv.FormatBool(*in.Webhook)); err != nil {
			return err
		}
	}
	if in.WebhookID != "" {
		if err := mw.WriteField("webhook_id", in.WebhookID); err != nil {
			return err
		}
	}
	if len(in.WebhookMetadata) > 0 {
		metadata, err := json.Marshal(in.WebhookMetadata)
		if err != nil {
			return fmt.Errorf("marshal webhook_metadata: %w", err)
		}
		if err := mw.WriteField("webhook_metadata", string(metadata)); err != nil {
			return err
		}
	}
	if in.FileFormat != "" {
		if err := mw.WriteField("file_format", in.FileFormat); err != nil {
			return err
		}
	}
	if in.Temperature != nil {
		if err := mw.WriteField("temperature", strconv.FormatFloat(*in.Temperature, 'f', -1, 64)); err != nil {
			return err
		}
	}
	if in.Seed != nil {
		if err := mw.WriteField("seed", strconv.Itoa(*in.Seed)); err != nil {
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
	for _, entity := range in.EntityDetection {
		if err := mw.WriteField("entity_detection", entity); err != nil {
			return err
		}
	}
	if in.UseSpeakerLibrary != nil {
		if err := mw.WriteField("use_speaker_library", strconv.FormatBool(*in.UseSpeakerLibrary)); err != nil {
			return err
		}
	}
	if in.DetectSpeakerRoles != nil {
		if err := mw.WriteField("detect_speaker_roles", strconv.FormatBool(*in.DetectSpeakerRoles)); err != nil {
			return err
		}
	}
	for _, entity := range in.EntityRedaction {
		if err := mw.WriteField("entity_redaction", entity); err != nil {
			return err
		}
	}
	if in.EntityRedactionMode != "" {
		if err := mw.WriteField("entity_redaction_mode", in.EntityRedactionMode); err != nil {
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
