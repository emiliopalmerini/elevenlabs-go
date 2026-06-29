package elevenlabs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"sort"
	"strconv"
)

type transcriptBody struct {
	newReader   func(attempt int) (io.Reader, error)
	contentType string
	retryable   bool
}

func createTranscriptBody(in CreateTranscriptRequest) transcriptBody {
	writer := multipart.NewWriter(io.Discard)
	boundary := writer.Boundary()
	contentType := writer.FormDataContentType()

	if in.File == nil {
		return transcriptBody{
			newReader: func(attempt int) (io.Reader, error) {
				return createTranscriptBufferedBody(in, boundary, attempt)
			},
			contentType: contentType,
			retryable:   true,
		}
	}

	if seeker, ok := in.File.Reader.(io.ReadSeeker); ok {
		return transcriptBody{
			newReader: func(attempt int) (io.Reader, error) {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return nil, fmt.Errorf("seek file: %w", err)
				}
				return createTranscriptStreamingBody(in, boundary, attempt)
			},
			contentType: contentType,
			retryable:   true,
		}
	}

	used := false
	return transcriptBody{
		newReader: func(attempt int) (io.Reader, error) {
			if used {
				return nil, errors.New("elevenlabs: transcript file reader is not replayable")
			}
			used = true
			return createTranscriptStreamingBody(in, boundary, attempt)
		},
		contentType: contentType,
		retryable:   false,
	}
}

func createTranscriptBufferedBody(in CreateTranscriptRequest, boundary string, attempt int) (io.Reader, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.SetBoundary(boundary); err != nil {
		return nil, err
	}
	if err := writeCreateTranscriptForm(mw, in, attempt); err != nil {
		_ = mw.Close()
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func createTranscriptStreamingBody(in CreateTranscriptRequest, boundary string, attempt int) (io.Reader, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	if err := mw.SetBoundary(boundary); err != nil {
		_ = pr.Close()
		_ = pw.CloseWithError(err)
		return nil, err
	}

	go func() {
		err := writeCreateTranscriptForm(mw, in, attempt)
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

func writeCreateTranscriptForm(mw *multipart.Writer, in CreateTranscriptRequest, attempt int) error {
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
		reader := in.File.Reader
		var progress *uploadProgressReader
		if in.OnUploadProgress != nil {
			progress = newUploadProgressReader(reader, uploadTotalBytes(in.File), attempt, in.OnUploadProgress)
			progress.reportProgress(false)
			reader = progress
		}
		if _, err := io.Copy(part, reader); err != nil {
			return fmt.Errorf("copy file: %w", err)
		}
		if progress != nil {
			progress.reportProgress(true)
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

type uploadProgressReader struct {
	reader  io.Reader
	sent    int64
	total   int64
	attempt int
	report  func(TranscriptUploadProgress)
}

func newUploadProgressReader(reader io.Reader, total int64, attempt int, report func(TranscriptUploadProgress)) *uploadProgressReader {
	if attempt <= 0 {
		attempt = 1
	}
	return &uploadProgressReader{
		reader:  reader,
		total:   total,
		attempt: attempt,
		report:  report,
	}
}

func (r *uploadProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.sent += int64(n)
		r.reportProgress(false)
	}
	return n, err
}

func (r *uploadProgressReader) reportProgress(done bool) {
	r.report(TranscriptUploadProgress{
		SentBytes:  r.sent,
		TotalBytes: r.total,
		Done:       done,
		Attempt:    r.attempt,
	})
}

func uploadTotalBytes(file *TranscriptFile) int64 {
	if file == nil || file.Reader == nil {
		return -1
	}
	if file.SizeBytes > 0 {
		return file.SizeBytes
	}
	if lenReader, ok := file.Reader.(interface{ Len() int }); ok {
		if length := lenReader.Len(); length >= 0 {
			return int64(length)
		}
	}
	if sizeReader, ok := file.Reader.(interface{ Size() int64 }); ok {
		if size := sizeReader.Size(); size >= 0 {
			return size
		}
	}
	if statReader, ok := file.Reader.(interface{ Stat() (fs.FileInfo, error) }); ok {
		info, err := statReader.Stat()
		if err == nil {
			return info.Size()
		}
	}
	return -1
}
