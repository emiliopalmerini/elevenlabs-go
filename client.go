package elevenlabs

import (
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

func (c *Client) do(ctx context.Context, build requestBuilder, retryable bool, out any) error {
	if c == nil {
		return errors.New("elevenlabs: nil client")
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
			return err
		}

		req, err := build(ctx)
		if err != nil {
			return err
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if attempt == maxAttempts {
				return err
			}
			if err := waitForRetry(ctx, cfg.backoffDelay(attempt)); err != nil {
				return err
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return decodeResponse(resp, out)
		}

		retryAfter := resp.Header.Get("Retry-After")
		apiErr, readErr := readAPIError(resp)
		if readErr != nil {
			if attempt < maxAttempts && cfg.retryableStatus(resp.StatusCode) {
				if err := waitForRetry(ctx, cfg.retryDelay(attempt, retryAfter)); err != nil {
					return err
				}
				continue
			}
			return readErr
		}

		if attempt == maxAttempts || !cfg.retryableStatus(apiErr.StatusCode) {
			return apiErr
		}
		if err := waitForRetry(ctx, cfg.retryDelay(attempt, retryAfter)); err != nil {
			return err
		}
	}

	return nil
}

func decodeResponse(resp *http.Response, out any) error {
	defer resp.Body.Close()

	if out == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("elevenlabs: decode response: %w", err)
	}

	return nil
}

func readAPIError(resp *http.Response) (*APIError, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: read error response: %w", err)
	}

	return newAPIError(resp.StatusCode, resp.Status, body), nil
}
