package elevenlabs

import "net/http"

// RawResponse contains HTTP response metadata.
//
// It intentionally does not include the response body.
type RawResponse struct {
	StatusCode int
	Status     string
	Header     http.Header
	URL        string
}

// Response contains parsed API data and HTTP response metadata.
type Response[T any] struct {
	Data        T
	RawResponse RawResponse
}

func newRawResponse(resp *http.Response) RawResponse {
	raw := RawResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header.Clone(),
	}
	if resp.Request != nil && resp.Request.URL != nil {
		raw.URL = resp.Request.URL.String()
	}
	return raw
}
