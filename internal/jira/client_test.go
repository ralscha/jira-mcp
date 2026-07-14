package jira

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := NewClient(srv.URL, "user@example.com", "tok", srv.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return c
}

func TestGetIssue(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/PROJ-1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user@example.com" || pass != "tok" {
			t.Fatalf("missing/invalid basic auth")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":  "10001",
			"key": "PROJ-1",
			"fields": map[string]any{
				"summary": "Test issue",
				"status":  map[string]any{"id": "1", "name": "To Do"},
			},
		})
	})

	issue, err := c.GetIssue(t.Context(), "PROJ-1", nil)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if issue.Key != "PROJ-1" || issue.Fields.Summary != "Test issue" {
		t.Errorf("GetIssue() = %+v", issue)
	}
}

func TestSearchIssues(t *testing.T) {
	requestCount := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount > 1 {
			t.Fatalf("SearchIssues made more than one request")
		}
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/search/jql" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["jql"] != "project = PROJ" {
			t.Fatalf("unexpected jql: %v", body["jql"])
		}
		if body["startAt"] != nil {
			t.Fatalf("unexpected startAt in request body: %v", body["startAt"])
		}
		if body["nextPageToken"] != nil {
			t.Fatalf("unexpected nextPageToken in first request: %v", body["nextPageToken"])
		}
		if body["maxResults"] != float64(50) {
			t.Fatalf("unexpected maxResults: %v", body["maxResults"])
		}
		fields, ok := body["fields"].([]any)
		if !ok {
			t.Fatalf("expected fields array, got %T", body["fields"])
		}
		wantFields := []string{"summary", "status", "issuetype", "project", "assignee", "reporter", "description", "created", "updated"}
		gotFields := make([]string, len(fields))
		for i, field := range fields {
			gotFields[i], ok = field.(string)
			if !ok {
				t.Fatalf("field %d has type %T", i, field)
			}
		}
		if !slices.Equal(gotFields, wantFields) {
			t.Fatalf("unexpected fields: %v", gotFields)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"isLast":        false,
			"nextPageToken": "page-2",
			"issues":        []map[string]any{{"id": "1", "key": "PROJ-1"}},
		})
	})

	result, err := c.SearchIssues(t.Context(), "project = PROJ", "", 50, nil)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if result.IsLast || result.NextPageToken != "page-2" || len(result.Issues) != 1 {
		t.Errorf("SearchIssues() = %+v", result)
	}
}

func TestSearchIssues_UsesNextPageToken(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/search/jql" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["maxResults"] != float64(2) {
			t.Fatalf("unexpected maxResults: %v", body["maxResults"])
		}
		if body["nextPageToken"] != "page-2" {
			t.Fatalf("unexpected nextPageToken: %v", body["nextPageToken"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"isLast": true,
			"issues": []map[string]any{
				{"id": "3", "key": "PROJ-3"},
				{"id": "4", "key": "PROJ-4"},
			},
		})
	})

	result, err := c.SearchIssues(t.Context(), "project = PROJ", "page-2", 2, []string{"summary"})
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if !result.IsLast || result.NextPageToken != "" || len(result.Issues) != 2 || result.Issues[0].Key != "PROJ-3" || result.Issues[1].Key != "PROJ-4" {
		t.Fatalf("unexpected issues: %+v", result.Issues)
	}
}

func TestCreateIssue(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		fields := body["fields"].(map[string]any)
		if fields["summary"] != "New issue" {
			t.Fatalf("unexpected fields: %v", fields)
		}
		if _, ok := fields["description"]; !ok {
			t.Fatalf("expected description to be set")
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10002", "key": "PROJ-2"})
	})

	issue, err := c.CreateIssue(t.Context(), CreateIssueInput{
		ProjectKey:  "PROJ",
		IssueType:   "Task",
		Summary:     "New issue",
		Description: "some details",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if issue.Key != "PROJ-2" {
		t.Errorf("CreateIssue() = %+v", issue)
	}
}

func TestUpdateIssue_NoFields(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when no fields are set")
	})
	err := c.UpdateIssue(t.Context(), "PROJ-1", UpdateIssueInput{})
	if err == nil {
		t.Fatal("UpdateIssue() error = nil, want error")
	}
}

func TestUpdateIssue(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/3/issue/PROJ-1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	summary := "Updated summary"
	err := c.UpdateIssue(t.Context(), "PROJ-1", UpdateIssueInput{Summary: &summary})
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
}

func TestGetTransitionsAndDoTransition(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/PROJ-1/transitions":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"transitions": []map[string]any{{"id": "11", "name": "Done"}},
				})
			} else {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				transition := body["transition"].(map[string]any)
				if transition["id"] != "11" {
					t.Fatalf("unexpected transition id: %v", transition["id"])
				}
				w.WriteHeader(http.StatusNoContent)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	transitions, err := c.GetTransitions(t.Context(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetTransitions() error = %v", err)
	}
	if len(transitions) != 1 || transitions[0].Name != "Done" {
		t.Fatalf("GetTransitions() = %+v", transitions)
	}

	if err := c.DoTransition(t.Context(), "PROJ-1", "11"); err != nil {
		t.Fatalf("DoTransition() error = %v", err)
	}
}

func TestAddComment(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue/PROJ-1/comment" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "999"})
	})
	comment, err := c.AddComment(t.Context(), "PROJ-1", "a comment")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if comment.ID != "999" {
		t.Errorf("AddComment() = %+v", comment)
	}
}

func TestListProjectsAndGetProject(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/project/search":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"startAt": 0, "maxResults": 50, "total": 1, "isLast": true,
				"values": []map[string]any{{"id": "1", "key": "PROJ", "name": "Project"}},
			})
		case "/rest/api/3/project/PROJ":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "1", "key": "PROJ", "name": "Project"})
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})

	projects, err := c.ListProjects(t.Context(), 0, 50)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects.Values) != 1 {
		t.Fatalf("ListProjects() = %+v", projects)
	}

	project, err := c.GetProject(t.Context(), "PROJ")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if project.Key != "PROJ" {
		t.Errorf("GetProject() = %+v", project)
	}
}

func TestAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errorMessages": []string{"Issue does not exist"},
		})
	})

	_, err := c.GetIssue(t.Context(), "NOPE-1", nil)
	if err == nil {
		t.Fatal("GetIssue() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if len(apiErr.Messages) != 1 || apiErr.Messages[0] != "Issue does not exist" {
		t.Errorf("Messages = %v", apiErr.Messages)
	}
}

func TestDownloadAndUploadAttachment(t *testing.T) {
	const fileContent = "hello attachment"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/attachment/10001":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "10001", "filename": "foo.txt", "mimeType": "text/plain",
				"size":    len(fileContent),
				"content": "http://" + r.Host + "/rest/api/3/attachment/content/10001",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/attachment/content/10001":
			_, _ = w.Write([]byte(fileContent))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue/PROJ-1/attachments":
			if r.Header.Get("X-Atlassian-Token") != "no-check" {
				t.Fatalf("missing X-Atlassian-Token header")
			}
			//nolint:gosec // 1<<20 (1 MiB) is a reasonable size limit for test attachments
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "10002", "filename": "bar.txt", "mimeType": "text/plain"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	downloaded, err := c.DownloadAttachment(t.Context(), "10001")
	if err != nil {
		t.Fatalf("DownloadAttachment() error = %v", err)
	}
	if downloaded.Filename != "foo.txt" {
		t.Errorf("Filename = %q", downloaded.Filename)
	}

	uploaded, err := c.UploadAttachment(t.Context(), "PROJ-1", "bar.txt", "text/plain", []byte("data"))
	if err != nil {
		t.Fatalf("UploadAttachment() error = %v", err)
	}
	if uploaded.ID != "10002" {
		t.Errorf("UploadAttachment() = %+v", uploaded)
	}
}
