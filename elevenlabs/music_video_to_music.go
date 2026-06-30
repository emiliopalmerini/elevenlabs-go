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
	"unicode/utf8"
)

const maxVideoToMusicUploadBytes = 200 * 1024 * 1024

// VideoToMusic generates background music from one or more video files.
func (s *MusicService) VideoToMusic(ctx context.Context, in VideoToMusicRequest) (*MusicComposition, error) {
	resp, err := s.VideoToMusicWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// VideoToMusicWithResponse generates background music from one or more video
// files and returns HTTP response metadata.
func (s *MusicService) VideoToMusicWithResponse(ctx context.Context, in VideoToMusicRequest) (*Response[*MusicComposition], error) {
	body, raw, err := s.doVideoToMusic(ctx, in)
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

func (s *MusicService) doVideoToMusic(ctx context.Context, in VideoToMusicRequest) ([]byte, RawResponse, error) {
	if err := validateVideoToMusicRequest(in); err != nil {
		return nil, RawResponse{}, err
	}
	core, err := s.apiClient()
	if err != nil {
		return nil, RawResponse{}, err
	}

	body := createVideoToMusicBody(in)
	attempt := 0
	build := func(ctx context.Context) (*http.Request, error) {
		attempt++
		reader, err := body.newReader(attempt)
		if err != nil {
			return nil, err
		}

		req, err := core.NewRequest(ctx, http.MethodPost, videoToMusicPath(in), reader)
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

func validateVideoToMusicRequest(in VideoToMusicRequest) error {
	if len(in.Videos) == 0 {
		return errors.New("elevenlabs: at least one video is required")
	}
	if len(in.Videos) > 10 {
		return errors.New("elevenlabs: videos must contain 10 files or fewer")
	}
	if utf8.RuneCountInString(in.Description) > 1000 {
		return errors.New("elevenlabs: description must be 1000 characters or fewer")
	}
	if len(in.Tags) > 10 {
		return errors.New("elevenlabs: tags must contain 10 values or fewer")
	}

	var knownTotal int64
	for i, video := range in.Videos {
		if strings.TrimSpace(video.Name) == "" {
			return fmt.Errorf("elevenlabs: video %d name is required", i)
		}
		if video.Reader == nil {
			return fmt.Errorf("elevenlabs: video %d reader is required", i)
		}
		if video.SizeBytes > 0 {
			if video.SizeBytes > maxVideoToMusicUploadBytes-knownTotal {
				return errors.New("elevenlabs: videos total size must be 200MB or fewer")
			}
			knownTotal += video.SizeBytes
		}
	}

	return nil
}

type videoToMusicBody struct {
	newReader   func(attempt int) (io.Reader, error)
	contentType string
	retryable   bool
}

func createVideoToMusicBody(in VideoToMusicRequest) videoToMusicBody {
	writer := multipart.NewWriter(io.Discard)
	boundary := writer.Boundary()
	contentType := writer.FormDataContentType()
	retryable := videoToMusicVideosReplayable(in.Videos)

	used := false
	return videoToMusicBody{
		newReader: func(attempt int) (io.Reader, error) {
			if used && !retryable {
				return nil, errors.New("elevenlabs: video readers are not replayable")
			}
			used = true
			if err := resetVideoToMusicReaders(in.Videos); err != nil {
				return nil, err
			}
			return createVideoToMusicStreamingBody(in, boundary)
		},
		contentType: contentType,
		retryable:   retryable,
	}
}

func videoToMusicVideosReplayable(videos []VideoToMusicFile) bool {
	for _, video := range videos {
		if _, ok := video.Reader.(io.ReadSeeker); !ok {
			return false
		}
	}
	return true
}

func resetVideoToMusicReaders(videos []VideoToMusicFile) error {
	for i, video := range videos {
		seeker, ok := video.Reader.(io.ReadSeeker)
		if !ok {
			continue
		}
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seek video %d: %w", i, err)
		}
	}
	return nil
}

func createVideoToMusicStreamingBody(in VideoToMusicRequest, boundary string) (io.Reader, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	if err := mw.SetBoundary(boundary); err != nil {
		_ = pr.Close()
		_ = pw.CloseWithError(err)
		return nil, err
	}

	go func() {
		err := writeVideoToMusicForm(mw, in)
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

func writeVideoToMusicForm(mw *multipart.Writer, in VideoToMusicRequest) error {
	for _, video := range in.Videos {
		part, err := mw.CreateFormFile("videos", video.Name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(part, video.Reader); err != nil {
			return fmt.Errorf("copy video: %w", err)
		}
	}
	if in.Description != "" {
		if err := mw.WriteField("description", in.Description); err != nil {
			return err
		}
	}
	for _, tag := range in.Tags {
		if err := mw.WriteField("tags", tag); err != nil {
			return err
		}
	}
	if in.ModelID != "" {
		if err := mw.WriteField("model_id", string(in.ModelID)); err != nil {
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

func videoToMusicPath(in VideoToMusicRequest) string {
	path := "/v1/music/video-to-music"

	values := url.Values{}
	setStringQuery(values, "output_format", in.OutputFormat)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
