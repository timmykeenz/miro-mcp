package miro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const baseURL = "https://api.miro.com/v2"

// Client is a thin Miro REST API client that authenticates via a static
// Bearer token read from the MIRO_ACCESS_TOKEN environment variable.
type Client struct {
	token      string
	httpClient *http.Client
}

// New creates a Client. It returns an error if MIRO_ACCESS_TOKEN is not set.
func New() (*Client, error) {
	token := os.Getenv("MIRO_ACCESS_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("MIRO_ACCESS_TOKEN environment variable is not set")
	}
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Get performs a GET request to /v2{path} with optional query params.
func (c *Client) Get(ctx context.Context, path string, params url.Values) ([]byte, int, error) {
	return c.do(ctx, http.MethodGet, path, params, nil)
}

// Post performs a POST request to /v2{path} with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, int, error) {
	return c.do(ctx, http.MethodPost, path, nil, body)
}

// Patch performs a PATCH request to /v2{path} with a JSON body.
func (c *Client) Patch(ctx context.Context, path string, body any) ([]byte, int, error) {
	return c.do(ctx, http.MethodPatch, path, nil, body)
}

// GetRaw fetches an arbitrary URL (e.g. image download URLs) and returns raw bytes.
func (c *Client) GetRaw(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("miro API error %d: %s", resp.StatusCode, truncate(data, 256))
	}
	return data, nil
}

func (c *Client) do(ctx context.Context, method, path string, params url.Values, body any) ([]byte, int, error) {
	u := baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("miro API error %d: %s", resp.StatusCode, truncate(data, 256))
	}
	return data, resp.StatusCode, nil
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// Unmarshal is a convenience helper.
func Unmarshal[T any](data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}
