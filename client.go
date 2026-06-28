package elevenlabs

import (
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

const (
	defaultBaseURL   = "https://api.elevenlabs.io"
	defaultUserAgent = "elevenlabs-go"
)

// Client is an ElevenLabs API client.
type Client struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	userAgent   string
	retryConfig retryConfig
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// NewClient creates a Client that authenticates with apiKey.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:      apiKey,
		baseURL:     defaultBaseURL,
		httpClient:  http.DefaultClient,
		userAgent:   defaultUserAgent,
		retryConfig: defaultRetryConfig(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}

	return c
}

// WithBaseURL overrides the ElevenLabs API base URL.
//
// This is primarily useful for tests.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if c == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, errors.New("elevenlabs: api key is required")
	}

	endpoint, err := c.endpoint(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)

	return req, nil
}

func (c *Client) endpoint(path string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("elevenlabs: parse base url: %w", err)
	}
	if !base.IsAbs() {
		return "", fmt.Errorf("elevenlabs: base url must be absolute: %q", c.baseURL)
	}

	relative, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return "", fmt.Errorf("elevenlabs: parse request path: %w", err)
	}

	return base.ResolveReference(relative).String(), nil
}

type requestBuilder func(context.Context) (*http.Request, error)

func (c *Client) do(ctx context.Context, build requestBuilder, retryable bool) ([]byte, RawResponse, error) {
	if c == nil {
		return nil, RawResponse{}, errors.New("elevenlabs: nil client")
	}

	cfg := c.retryConfig
	if cfg.maxAttempts == 0 {
		cfg = defaultRetryConfig()
	}

	maxAttempts := 1
	if retryable {
		maxAttempts = cfg.maxAttempts
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, RawResponse{}, err
		}

		req, err := build(ctx)
		if err != nil {
			return nil, RawResponse{}, err
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, RawResponse{}, ctxErr
			}
			if attempt == maxAttempts {
				return nil, RawResponse{}, err
			}
			if err := waitForRetry(ctx, cfg.backoffDelay(attempt)); err != nil {
				return nil, RawResponse{}, err
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body, raw, err := readResponse(resp)
			return body, raw, err
		}

		retryAfter := resp.Header.Get("Retry-After")
		apiErr, raw, readErr := readAPIError(resp)
		if readErr != nil {
			if attempt < maxAttempts && cfg.retryableStatus(resp.StatusCode) {
				if err := waitForRetry(ctx, cfg.retryDelay(attempt, retryAfter)); err != nil {
					return nil, raw, err
				}
				continue
			}
			return nil, raw, readErr
		}

		if attempt == maxAttempts || !cfg.retryableStatus(apiErr.StatusCode) {
			return nil, raw, apiErr
		}
		if err := waitForRetry(ctx, cfg.retryDelay(attempt, retryAfter)); err != nil {
			return nil, raw, err
		}
	}

	return nil, RawResponse{}, nil
}

func readResponse(resp *http.Response) ([]byte, RawResponse, error) {
	defer resp.Body.Close()

	raw := newRawResponse(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, raw, fmt.Errorf("elevenlabs: read response: %w", err)
	}
	return body, raw, nil
}

func decodeResponse(body []byte, out any) error {
	if out == nil {
		return nil
	}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(out); err != nil {
		return fmt.Errorf("elevenlabs: decode response: %w", err)
	}

	return nil
}

func decodeOptionalResponse(body []byte) (any, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}
	var out any
	if err := decodeResponse(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func readAPIError(resp *http.Response) (*APIError, RawResponse, error) {
	defer resp.Body.Close()

	raw := newRawResponse(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, raw, fmt.Errorf("elevenlabs: read error response: %w", err)
	}

	return newAPIError(resp.StatusCode, resp.Status, body), raw, nil
}
