package elevenlabs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// SeparateStems separates an audio file into individual stems and returns the
// ZIP archive bytes.
func (s *MusicService) SeparateStems(ctx context.Context, in SeparateStemsRequest) ([]byte, error) {
	resp, err := s.SeparateStemsWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// SeparateStemsWithResponse separates an audio file into individual stems,
// returns ZIP archive bytes, and includes HTTP response metadata.
func (s *MusicService) SeparateStemsWithResponse(ctx context.Context, in SeparateStemsRequest) (*Response[[]byte], error) {
	body, raw, err := s.doSeparateStems(ctx, in)
	if err != nil {
		return nil, err
	}
	return &Response[[]byte]{
		Data:        body,
		RawResponse: raw,
	}, nil
}

func (s *MusicService) doSeparateStems(ctx context.Context, in SeparateStemsRequest) ([]byte, RawResponse, error) {
	if err := validateSeparateStemsRequest(in); err != nil {
		return nil, RawResponse{}, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, RawResponse{}, err
	}

	body := createSeparateStemsBody(in)
	build := func(ctx context.Context) (*http.Request, error) {
		reader, err := body.newReader()
		if err != nil {
			return nil, err
		}

		req, err := core.NewRequest(ctx, http.MethodPost, separateStemsPath(in), reader)
		if err != nil {
			if closer, ok := reader.(io.Closer); ok {
				_ = closer.Close()
			}
			return nil, err
		}
		req.Header.Set("Content-Type", body.contentType)
		return req, nil
	}

	return core.Do(ctx, build, body.retryable)
}

func validateSeparateStemsRequest(in SeparateStemsRequest) error {
	if strings.TrimSpace(in.File.Name) == "" {
		return errors.New("elevenlabs: file name is required")
	}
	if in.File.Reader == nil {
		return errors.New("elevenlabs: file reader is required")
	}
	return nil
}

type separateStemsBody struct {
	newReader   func() (io.Reader, error)
	contentType string
	retryable   bool
}

func createSeparateStemsBody(in SeparateStemsRequest) separateStemsBody {
	writer := multipart.NewWriter(io.Discard)
	boundary := writer.Boundary()
	contentType := writer.FormDataContentType()

	if seeker, ok := in.File.Reader.(io.ReadSeeker); ok {
		return separateStemsBody{
			newReader: func() (io.Reader, error) {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return nil, fmt.Errorf("seek file: %w", err)
				}
				return createSeparateStemsStreamingBody(in, boundary)
			},
			contentType: contentType,
			retryable:   true,
		}
	}

	used := false
	return separateStemsBody{
		newReader: func() (io.Reader, error) {
			if used {
				return nil, errors.New("elevenlabs: stem separation file reader is not replayable")
			}
			used = true
			return createSeparateStemsStreamingBody(in, boundary)
		},
		contentType: contentType,
		retryable:   false,
	}
}

func createSeparateStemsStreamingBody(in SeparateStemsRequest, boundary string) (io.Reader, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	if err := mw.SetBoundary(boundary); err != nil {
		_ = pr.Close()
		_ = pw.CloseWithError(err)
		return nil, err
	}

	go func() {
		err := writeSeparateStemsForm(mw, in)
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

func writeSeparateStemsForm(mw *multipart.Writer, in SeparateStemsRequest) error {
	part, err := mw.CreateFormFile("file", in.File.Name)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, in.File.Reader); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	if in.StemVariationID != "" {
		if err := mw.WriteField("stem_variation_id", string(in.StemVariationID)); err != nil {
			return err
		}
	}
	if in.SignWithC2PA != nil {
		if err := mw.WriteField("sign_with_c2pa", strconv.FormatBool(*in.SignWithC2PA)); err != nil {
			return err
		}
	}
	return nil
}

func separateStemsPath(in SeparateStemsRequest) string {
	path := "/v1/music/stem-separation"

	values := url.Values{}
	setStringQuery(values, "output_format", in.OutputFormat)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
