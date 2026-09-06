// Package jira provides a minimal client for the Jira Cloud REST API (v3),
// covering the subset of endpoints needed by the jira-mcp server.
package jira

import (
	"bytes"
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
	"strconv"
	"strings"
	"time"
)

const (
	// MaxAttachmentBytes caps how much attachment data is read from or sent
	// to Jira in a single call, so a large attachment cannot exhaust memory
	// or overflow the MCP client's context.
	MaxAttachmentBytes = 25 << 20 // 25 MiB

	// maxJSONResponseBytes caps how large a JSON response body may be.
	maxJSONResponseBytes = 16 << 20 // 16 MiB

	// maxRetries is the number of additional attempts made after a
	// retryable response (429 or a transient 5xx).
	maxRetries = 3

	// maxRetryDelay caps how long a single Retry-After hint is honoured.
	maxRetryDelay = 30 * time.Second
)

// ErrTooLarge reports that a payload exceeded the configured size limit.
var ErrTooLarge = errors.New("jira: payload exceeds size limit")

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
	if !u.IsAbs() || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("jira: base URL must be absolute and must not contain user information, a query, or a fragment")
	}
	// Resolve API paths beneath the complete configured path. Without a
	// trailing slash, url.ResolveReference treats the final path segment as a
	// file and drops it, which breaks scoped gateway URLs ending in a cloud id.
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
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

// do sends req, retrying transient failures, and returns the response
// together with its body read up to maxBytes.
func (c *Client) do(req *http.Request, maxBytes int64) (*http.Response, []byte, error) {
	for attempt := 0; ; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("jira: request failed: %w", err)
		}

		body, readErr := readLimited(resp.Body, maxBytes)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, nil, readErr
		}

		if attempt >= maxRetries || !isRetryable(resp.StatusCode) {
			return resp, body, nil
		}

		select {
		case <-req.Context().Done():
			return nil, nil, fmt.Errorf("jira: request failed: %w", req.Context().Err())
		case <-time.After(retryDelay(resp, attempt)):
		}

		if req.GetBody != nil {
			rewound, err := req.GetBody()
			if err != nil {
				return nil, nil, fmt.Errorf("jira: rewinding request body for retry: %w", err)
			}
			req.Body = rewound
		}
	}
}

// isRetryable reports whether a status code is worth retrying: Jira's rate
// limit response and transient gateway errors.
func isRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// retryDelay honours a Retry-After header when present, and otherwise backs
// off exponentially.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && seconds >= 0 {
			return min(time.Duration(seconds)*time.Second, maxRetryDelay)
		}
		if at, err := http.ParseTime(v); err == nil {
			if d := time.Until(at); d > 0 {
				return min(d, maxRetryDelay)
			}
			return 0
		}
	}
	return min(500*time.Millisecond<<attempt, maxRetryDelay)
}

// readLimited reads up to maxBytes from r, returning ErrTooLarge if more
// data is available.
func readLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("jira: reading response body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: response larger than %d bytes", ErrTooLarge, maxBytes)
	}
	return body, nil
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

	resp, respBody, err := c.do(req, maxJSONResponseBytes)
	if err != nil {
		return err
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
// attachment content, and refuses bodies larger than maxBytes.
func (c *Client) doRaw(ctx context.Context, path string, maxBytes int64) ([]byte, string, error) {
	req, err := c.newRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, "", err
	}

	resp, respBody, err := c.do(req, maxBytes)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", parseAPIError(resp.StatusCode, respBody)
	}

	return respBody, resp.Header.Get("Content-Type"), nil
}

// doMultipart sends a multipart/form-data POST request with a single file
// part named "file", decoding a JSON array response into out.
func (c *Client) doMultipart(ctx context.Context, path, filename, mimeType string, data []byte, out any) error {
	if int64(len(data)) > MaxAttachmentBytes {
		return fmt.Errorf("%w: attachment is %d bytes, limit is %d", ErrTooLarge, len(data), int64(MaxAttachmentBytes))
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": filename,
	}))
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

	req, err := c.newRequest(ctx, "POST", path, nil, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	// Required by Jira for attachment uploads to bypass XSRF checks.
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, respBody, err := c.do(req, maxJSONResponseBytes)
	if err != nil {
		return err
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
		if parsed.User != nil || !strings.EqualFold(parsed.Scheme, c.baseURL.Scheme) ||
			!strings.EqualFold(parsed.Host, c.baseURL.Host) {
			return nil, fmt.Errorf("jira: refusing to request %q: origin does not match configured Jira base URL", ref)
		}
		if basePath := c.baseURL.EscapedPath(); basePath != "/" &&
			!strings.HasPrefix(parsed.EscapedPath(), basePath) {
			return nil, fmt.Errorf("jira: refusing to request %q: path is outside configured Jira base URL", ref)
		}
		return parsed, nil
	}
	if parsed.Host != "" {
		return nil, fmt.Errorf("jira: refusing to request %q: URL has an unexpected host", ref)
	}
	parsed.Path = strings.TrimPrefix(parsed.Path, "/")
	parsed.RawPath = strings.TrimPrefix(parsed.RawPath, "/")
	return c.baseURL.ResolveReference(parsed), nil
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
