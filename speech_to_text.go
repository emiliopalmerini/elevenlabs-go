package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SpeechToTextService provides speech-to-text API methods.
type SpeechToTextService struct {
	client *Client
}

// SpeechToTextRequest configures a speech-to-text conversion.
type SpeechToTextRequest struct {
	ModelID string

	Audio    io.Reader
	FileName string
	FileSize int64

	SourceURL       string
	CloudStorageURL string
	EnableLogging   *bool

	LanguageCode            string
	TagAudioEvents          *bool
	NumSpeakers             *int
	TimestampsGranularity   string
	Diarize                 *bool
	DiarizationThreshold    *float64
	AdditionalFormats       []AdditionalFormat
	FileFormat              string
	WebhookID               string
	Temperature             *float64
	Seed                    *int
	UseMultiChannel         *bool
	MultichannelOutputStyle string
	WebhookMetadata         map[string]any
	EntityDetection         EntitySelector
	NoVerbatim              *bool
	UseSpeakerLibrary       *bool
	DetectSpeakerRoles      *bool
	EntityRedaction         EntitySelector
	EntityRedactionMode     string
	Keyterms                []string

	OnUploadProgress func(UploadProgress)
}

// AdditionalFormat is encoded as one item in the additional_formats multipart
// field. It is intentionally flexible because ElevenLabs supports several
// export option shapes.
type AdditionalFormat map[string]any

// EntitySelector represents entity detection or redaction selectors. A single
// value is sent as a string; multiple values are sent as a JSON array.
type EntitySelector []string

// UploadProgress reports streamed file upload progress.
type UploadProgress struct {
	SentBytes  int64
	TotalBytes int64
}

// Convert transcribes audio synchronously.
func (s *SpeechToTextService) Convert(ctx context.Context, req *SpeechToTextRequest) (*TranscriptResponse, error) {
	var out TranscriptResponse
	if err := s.convert(ctx, req, false, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ConvertFile opens path and transcribes it synchronously.
func (s *SpeechToTextService) ConvertFile(ctx context.Context, path string, req *SpeechToTextRequest) (*TranscriptResponse, error) {
	var out TranscriptResponse
	if err := s.convert(ctx, req, false, fileBodySource{path: path}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SubmitWebhook starts an asynchronous transcription request.
func (s *SpeechToTextService) SubmitWebhook(ctx context.Context, req *SpeechToTextRequest) (*WebhookResponse, error) {
	var out WebhookResponse
	if err := s.convert(ctx, req, true, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SubmitWebhookFile opens path and starts an asynchronous transcription request.
func (s *SpeechToTextService) SubmitWebhookFile(ctx context.Context, path string, req *SpeechToTextRequest) (*WebhookResponse, error) {
	var out WebhookResponse
	if err := s.convert(ctx, req, true, fileBodySource{path: path}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTranscript fetches a stored transcript by ID.
func (s *SpeechToTextService) GetTranscript(ctx context.Context, id string) (*TranscriptResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("elevenlabs: transcript ID is required")
	}
	var out TranscriptResponse
	path := "/v1/speech-to-text/transcripts/" + url.PathEscape(id)
	err := s.client.doJSON(ctx, http.MethodGet, path, nil, true, func(ctx context.Context) (*http.Request, error) {
		return s.client.newRequest(ctx, http.MethodGet, path, nil, nil)
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTranscript deletes a stored transcript by ID.
func (s *SpeechToTextService) DeleteTranscript(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("elevenlabs: transcript ID is required")
	}
	path := "/v1/speech-to-text/transcripts/" + url.PathEscape(id)
	return s.client.doJSON(ctx, http.MethodDelete, path, nil, true, func(ctx context.Context) (*http.Request, error) {
		return s.client.newRequest(ctx, http.MethodDelete, path, nil, nil)
	}, nil)
}

func (s *SpeechToTextService) convert(ctx context.Context, req *SpeechToTextRequest, webhook bool, source bodySource, out any) error {
	if req == nil {
		req = &SpeechToTextRequest{}
	}
	if err := validateSpeechToTextRequest(req, source); err != nil {
		return err
	}
	query := url.Values{}
	if req.EnableLogging != nil {
		query.Set("enable_logging", strconv.FormatBool(*req.EnableLogging))
	}
	replayable := source == nil && req.Audio == nil
	if source != nil {
		replayable = source.replayable()
	}
	path := "/v1/speech-to-text"
	return s.client.doJSON(ctx, http.MethodPost, path, query, replayable, func(ctx context.Context) (*http.Request, error) {
		body, contentType, err := s.buildMultipartBody(req, webhook, source)
		if err != nil {
			return nil, err
		}
		httpReq, err := s.client.newRequest(ctx, http.MethodPost, path, query, body)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", contentType)
		return httpReq, nil
	}, out)
}

func validateSpeechToTextRequest(req *SpeechToTextRequest, source bodySource) error {
	if strings.TrimSpace(req.ModelID) == "" {
		return errors.New("elevenlabs: speech-to-text ModelID is required")
	}
	if source != nil {
		if err := source.validate(); err != nil {
			return err
		}
	}
	sources := 0
	if source != nil {
		sources++
	}
	if req.Audio != nil {
		sources++
	}
	if strings.TrimSpace(req.SourceURL) != "" {
		sources++
	}
	if strings.TrimSpace(req.CloudStorageURL) != "" {
		sources++
	}
	if sources != 1 {
		return errors.New("elevenlabs: provide exactly one of Audio, ConvertFile path, SourceURL, or CloudStorageURL")
	}
	if req.NumSpeakers != nil && (*req.NumSpeakers < 1 || *req.NumSpeakers > 32) {
		return errors.New("elevenlabs: NumSpeakers must be between 1 and 32")
	}
	if req.Seed != nil && (*req.Seed < 0 || *req.Seed > 2147483647) {
		return errors.New("elevenlabs: Seed must be between 0 and 2147483647")
	}
	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return errors.New("elevenlabs: Temperature must be between 0.0 and 2.0")
	}
	return nil
}

func (s *SpeechToTextService) buildMultipartBody(req *SpeechToTextRequest, webhook bool, source bodySource) (io.Reader, string, error) {
	fields, err := speechToTextFields(req, webhook)
	if err != nil {
		return nil, "", err
	}
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		err := writeSpeechToTextMultipart(writer, req, fields, source)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := writer.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	return pr, writer.FormDataContentType(), nil
}

func writeSpeechToTextMultipart(writer *multipart.Writer, req *SpeechToTextRequest, fields []formField, source bodySource) error {
	for _, field := range fields {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return fmt.Errorf("elevenlabs: write multipart field %q: %w", field.name, err)
		}
	}
	if source != nil || req.Audio != nil {
		return writeSpeechToTextFile(writer, req, source)
	}
	return nil
}

type formField struct {
	name  string
	value string
}

func speechToTextFields(req *SpeechToTextRequest, webhook bool) ([]formField, error) {
	fields := []formField{{name: "model_id", value: req.ModelID}}
	addStringField(&fields, "language_code", req.LanguageCode)
	addBoolField(&fields, "tag_audio_events", req.TagAudioEvents)
	addIntField(&fields, "num_speakers", req.NumSpeakers)
	addStringField(&fields, "timestamps_granularity", req.TimestampsGranularity)
	addBoolField(&fields, "diarize", req.Diarize)
	addFloatField(&fields, "diarization_threshold", req.DiarizationThreshold)
	addJSONField(&fields, "additional_formats", req.AdditionalFormats)
	addStringField(&fields, "file_format", req.FileFormat)
	addStringField(&fields, "cloud_storage_url", req.CloudStorageURL)
	addStringField(&fields, "source_url", req.SourceURL)
	if webhook {
		fields = append(fields, formField{name: "webhook", value: "true"})
	}
	addStringField(&fields, "webhook_id", req.WebhookID)
	addFloatField(&fields, "temperature", req.Temperature)
	addIntField(&fields, "seed", req.Seed)
	addBoolField(&fields, "use_multi_channel", req.UseMultiChannel)
	addStringField(&fields, "multichannel_output_style", req.MultichannelOutputStyle)
	addJSONField(&fields, "webhook_metadata", req.WebhookMetadata)
	addEntitySelectorField(&fields, "entity_detection", req.EntityDetection)
	addBoolField(&fields, "no_verbatim", req.NoVerbatim)
	addBoolField(&fields, "use_speaker_library", req.UseSpeakerLibrary)
	addBoolField(&fields, "detect_speaker_roles", req.DetectSpeakerRoles)
	addEntitySelectorField(&fields, "entity_redaction", req.EntityRedaction)
	addStringField(&fields, "entity_redaction_mode", req.EntityRedactionMode)
	for _, term := range req.Keyterms {
		if strings.TrimSpace(term) != "" {
			fields = append(fields, formField{name: "keyterms", value: term})
		}
	}
	return fields, nil
}

func addStringField(fields *[]formField, name, value string) {
	if strings.TrimSpace(value) != "" {
		*fields = append(*fields, formField{name: name, value: value})
	}
}

func addBoolField(fields *[]formField, name string, value *bool) {
	if value != nil {
		*fields = append(*fields, formField{name: name, value: strconv.FormatBool(*value)})
	}
}

func addIntField(fields *[]formField, name string, value *int) {
	if value != nil {
		*fields = append(*fields, formField{name: name, value: strconv.Itoa(*value)})
	}
}

func addFloatField(fields *[]formField, name string, value *float64) {
	if value != nil {
		*fields = append(*fields, formField{name: name, value: strconv.FormatFloat(*value, 'f', -1, 64)})
	}
}

func addJSONField(fields *[]formField, name string, value any) error {
	if isZeroJSONValue(value) {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("elevenlabs: encode %s: %w", name, err)
	}
	*fields = append(*fields, formField{name: name, value: string(encoded)})
	return nil
}

func addEntitySelectorField(fields *[]formField, name string, selector EntitySelector) error {
	switch len(selector) {
	case 0:
		return nil
	case 1:
		*fields = append(*fields, formField{name: name, value: selector[0]})
		return nil
	default:
		encoded, err := json.Marshal([]string(selector))
		if err != nil {
			return fmt.Errorf("elevenlabs: encode %s: %w", name, err)
		}
		*fields = append(*fields, formField{name: name, value: string(encoded)})
		return nil
	}
}

func isZeroJSONValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case []AdditionalFormat:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

func writeSpeechToTextFile(writer *multipart.Writer, req *SpeechToTextRequest, source bodySource) error {
	var (
		reader   io.Reader
		closeFn  func() error
		filename = req.FileName
		size     = req.FileSize
		err      error
	)
	if source != nil {
		reader, closeFn, filename, size, err = source.open()
		if err != nil {
			return err
		}
	} else {
		reader = req.Audio
		closeFn = func() error { return nil }
	}
	defer func() { _ = closeFn() }()

	if strings.TrimSpace(filename) == "" {
		filename = "audio"
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(filepath.Base(filename))))
	partHeader.Set("Content-Type", contentTypeFor(filename))
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return fmt.Errorf("elevenlabs: create multipart file part: %w", err)
	}
	if req.OnUploadProgress != nil {
		req.OnUploadProgress(UploadProgress{SentBytes: 0, TotalBytes: size})
		reader = &progressReader{
			reader: reader,
			total:  size,
			report: req.OnUploadProgress,
		}
	}
	if _, err := io.Copy(part, reader); err != nil {
		return fmt.Errorf("elevenlabs: stream audio file: %w", err)
	}
	return nil
}

type bodySource interface {
	validate() error
	open() (io.Reader, func() error, string, int64, error)
	replayable() bool
}

type fileBodySource struct {
	path string
}

func (s fileBodySource) open() (io.Reader, func() error, string, int64, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, nil, "", 0, fmt.Errorf("elevenlabs: open audio file %s: %w", s.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, "", 0, fmt.Errorf("elevenlabs: inspect audio file %s: %w", s.path, err)
	}
	return file, file.Close, filepath.Base(s.path), info.Size(), nil
}

func (s fileBodySource) validate() error {
	if strings.TrimSpace(s.path) == "" {
		return errors.New("elevenlabs: file path is required")
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("elevenlabs: inspect audio file %s: %w", s.path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("elevenlabs: audio file %s is a directory", s.path)
	}
	return nil
}

func (s fileBodySource) replayable() bool { return true }

type progressReader struct {
	reader io.Reader
	sent   int64
	total  int64
	report func(UploadProgress)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.sent += int64(n)
		r.report(UploadProgress{SentBytes: r.sent, TotalBytes: r.total})
	}
	return n, err
}

func escapeQuotes(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(s)
}

func contentTypeFor(filename string) string {
	if typ := mime.TypeByExtension(filepath.Ext(filename)); typ != "" {
		return typ
	}
	return "application/octet-stream"
}
