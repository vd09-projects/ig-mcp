// Package graphapi provides a low-level HTTP client for the Facebook Graph API.
// It handles authentication, request building, and error parsing.
// Domain-specific logic (Instagram Reels, etc.) belongs in higher-level packages.
package graphapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thin HTTP wrapper for the Facebook Graph API. It handles
// authentication headers, form-encoded POSTs, query-string GETs, and
// structured error parsing.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	accessToken string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient injects a custom *http.Client (e.g. for timeouts or testing).
func WithHTTPClient(c *http.Client) Option {
	return func(g *Client) { g.httpClient = c }
}

// NewClient creates a Graph API client.
//
//	baseURL:     "https://graph.facebook.com/v21.0"
//	accessToken: long-lived page/user token
func NewClient(baseURL, accessToken string, opts ...Option) *Client {
	c := &Client{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		baseURL:     strings.TrimRight(baseURL, "/"),
		accessToken: accessToken,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Post sends a JSON POST and returns the raw JSON body.
func (c *Client) Post(ctx context.Context, endpoint string, form url.Values) (json.RawMessage, error) {
	// Convert url.Values to a map for JSON encoding.
	// Multi-value keys (e.g. carousel children) become JSON arrays.
	body := make(map[string]any, len(form))
	for k, v := range form {
		if len(v) == 1 {
			body[k] = v[0]
		} else if len(v) > 1 {
			body[k] = v
		}
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding POST body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+endpoint,
		bytes.NewReader(b),
	)
	if err != nil {
		return nil, fmt.Errorf("building POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	return c.do(req)
}

// Get sends a GET with query params and returns the raw JSON body.
func (c *Client) Get(ctx context.Context, endpoint string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	}

	u := c.baseURL + endpoint + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	return c.do(req)
}

func (c *Client) do(req *http.Request) (json.RawMessage, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var raw rawAPIError
		if json.Unmarshal(body, &raw) == nil && raw.Error.Message != "" {
			return nil, &APIError{
				HTTPStatus: resp.StatusCode,
				Type:       raw.Error.Type,
				Message:    raw.Error.Message,
				Code:       raw.Error.Code,
			}
		}
		return nil, &APIError{
			HTTPStatus: resp.StatusCode,
			Message:    string(body),
		}
	}

	return body, nil
}

// ExtractID is a convenience that extracts the "id" field from a JSON response.
func ExtractID(raw json.RawMessage) (string, error) {
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parsing id: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("empty id in response: %s", string(raw))
	}
	return result.ID, nil
}
