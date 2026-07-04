// Package jira provides a minimal client for the Jira Cloud REST API (v3),
// covering the subset of endpoints needed by the jira-mcp server.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

// Client is a Jira Cloud REST API v3 client authenticated via Basic auth
// using an account email and API token.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	email      string
	token      string
}

// NewClient creates a Client for the given Jira Cloud base URL (e.g.
// "https://your-domain.atlassian.net"). If httpClient is nil,
// http.DefaultClient is used.
func NewClient(baseURL, email, token string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("jira: invalid base URL: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    u,
		email:      email,
		token:      token,
	}, nil
}

// APIError represents a non-2xx response from the Jira REST API.
type APIError struct {
	StatusCode int
	Messages   []string
	Errors     map[string]string
}

func (e *APIError) Error() string {
	parts := make([]string, 0, len(e.Messages)+len(e.Errors))
	parts = append(parts, e.Messages...)
	for field, msg := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("jira: request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("jira: request failed with status %d: %s", e.StatusCode, strings.Join(parts, "; "))
}

// doJSON sends a request with an optional JSON-encoded body and decodes a
// JSON response into out (if non-nil). path is resolved relative to the
// client's base URL.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("jira: encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := c.newRequest(ctx, method, path, query, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("jira: reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("jira: decoding response body: %w", err)
		}
	}
	return nil
}

// doRaw sends a GET request and returns the raw response body along with
// its Content-Type header. It is used for non-JSON payloads such as
// attachment content.
func (c *Client) doRaw(ctx context.Context, path string) ([]byte, string, error) {
	req, err := c.newRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("jira: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("jira: reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", parseAPIError(resp.StatusCode, respBody)
	}

	return respBody, resp.Header.Get("Content-Type"), nil
}

// doMultipart sends a multipart/form-data POST request with a single file
// part named "file", decoding a JSON array response into out.
func (c *Client) doMultipart(ctx context.Context, path, filename, mimeType string, data []byte, out any) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	if mimeType != "" {
		partHeader.Set("Content-Type", mimeType)
	}
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return fmt.Errorf("jira: creating multipart part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("jira: writing multipart data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("jira: closing multipart writer: %w", err)
	}

	req, err := c.newRequest(ctx, "POST", path, nil, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	// Required by Jira for attachment uploads to bypass XSRF checks.
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("jira: reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("jira: decoding response body: %w", err)
		}
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u, err := c.resolveURL(path)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("jira: building request: %w", err)
	}
	req.SetBasicAuth(c.email, c.token)
	return req, nil
}

// resolveURL resolves ref against the client's base URL. If ref is already
// an absolute URL (e.g. an attachment "content" link returned by the API),
// it is validated and returned as-is, provided it shares the base URL's
// host, to avoid inadvertently sending credentials to an unrelated host.
func (c *Client) resolveURL(ref string) (*url.URL, error) {
	parsed, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("jira: invalid URL %q: %w", ref, err)
	}
	if parsed.IsAbs() {
		if parsed.Host != c.baseURL.Host {
			return nil, fmt.Errorf("jira: refusing to request %q: host does not match configured Jira base URL", ref)
		}
		return parsed, nil
	}
	return c.baseURL.ResolveReference(&url.URL{Path: strings.TrimPrefix(ref, "/")}), nil
}

func parseAPIError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode}
	var payload struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Messages = payload.ErrorMessages
		apiErr.Errors = payload.Errors
	}
	if len(apiErr.Messages) == 0 && len(apiErr.Errors) == 0 && len(body) > 0 {
		apiErr.Messages = []string{strings.TrimSpace(string(body))}
	}
	return apiErr
}
