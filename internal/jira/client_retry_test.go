package jira

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClient_RetriesRateLimited(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1"})
	})

	issue, err := c.GetIssue(t.Context(), "PROJ-1", nil)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if issue.Key != "PROJ-1" {
		t.Errorf("GetIssue() = %+v", issue)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("server calls = %d, want 2", got)
	}
}

func TestClient_RetriesResendRequestBody(t *testing.T) {
	var calls atomic.Int32
	var lastBody string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if jql, ok := body["jql"].(string); ok {
			lastBody = jql
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"isLast": true})
	})

	if _, err := c.SearchIssues(t.Context(), "project = PROJ", "", 10, nil); err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("server calls = %d, want 2", calls.Load())
	}
	if lastBody != "project = PROJ" {
		t.Errorf("retried request body jql = %q, want the original query", lastBody)
	}
}

func TestClient_GivesUpAfterMaxRetries(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := c.GetIssue(t.Context(), "PROJ-1", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("GetIssue() error = %v, want 429 APIError", err)
	}
	if got := calls.Load(); got != maxRetries+1 {
		t.Errorf("server calls = %d, want %d", got, maxRetries+1)
	}
}

func TestDownloadAttachment_RejectsOversizedAttachment(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       "1",
			"filename": "huge.bin",
			"size":     int64(MaxAttachmentBytes) + 1,
			"content":  "http://example.invalid/content",
		})
	})

	_, err := c.DownloadAttachment(t.Context(), "1")
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("DownloadAttachment() error = %v, want ErrTooLarge", err)
	}
}

func TestUploadAttachment_RejectsOversizedData(t *testing.T) {
	c := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("upload should be rejected before any request is sent")
	})

	data := strings.Repeat("x", MaxAttachmentBytes+1)
	_, err := c.UploadAttachment(t.Context(), "PROJ-1", "huge.bin", "application/octet-stream", []byte(data))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("UploadAttachment() error = %v, want ErrTooLarge", err)
	}
}
