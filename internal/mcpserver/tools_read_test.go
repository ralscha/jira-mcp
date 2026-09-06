package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jira-mcp/internal/jira"
)

func TestDownloadAttachmentReturnsNativeImageContent(t *testing.T) {
	const imageData = "\x89PNG\r\n\x1a\nimage-data"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/attachment/10001":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       10001,
				"filename": "screenshot.png",
				"mimeType": "image/png",
				"size":     len(imageData),
				"content":  "http://" + r.Host + "/attachment-content/10001",
			})
		case "/attachment-content/10001":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte(imageData))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := jira.NewClient(srv.URL, "user@example.com", "tok", srv.Client())
	if err != nil {
		t.Fatalf("jira.NewClient() error = %v", err)
	}

	result, output, err := downloadAttachment(client)(
		t.Context(), nil, DownloadAttachmentInput{AttachmentID: "10001"},
	)
	if err != nil {
		t.Fatalf("downloadAttachment() error = %v", err)
	}
	if output.Filename != "screenshot.png" || output.MimeType != "image/png" {
		t.Errorf("output metadata = %+v", output)
	}
	if output.DataBase64 != "" {
		t.Errorf("DataBase64 should be omitted for native image content")
	}
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("result content = %+v, want one image", result)
	}

	image, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.ImageContent", result.Content[0])
	}
	if image.MIMEType != "image/png" {
		t.Errorf("MIMEType = %q, want image/png", image.MIMEType)
	}
	if string(image.Data) != imageData {
		t.Errorf("image data = %q", image.Data)
	}
}

func TestIssueToSummaryIncludesAdditionalFields(t *testing.T) {
	issue := &jira.Issue{
		Key: "PROJ-1",
		Fields: jira.IssueFields{
			Additional: map[string]any{"customfield_10011": "customer-facing"},
		},
	}
	summary := issueToSummary(issue)
	if summary.Fields["customfield_10011"] != "customer-facing" {
		t.Fatalf("summary fields = %v", summary.Fields)
	}
}
