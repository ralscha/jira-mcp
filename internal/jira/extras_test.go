package jira

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestListComments(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/PROJ-1/comment" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("startAt"); got != "10" {
			t.Fatalf("unexpected startAt: %q", got)
		}
		if got := r.URL.Query().Get("maxResults"); got != "50" {
			t.Fatalf("unexpected maxResults: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"startAt":    10,
			"maxResults": 50,
			"total":      12,
			"comments": []map[string]any{{
				"id":     "100",
				"author": map[string]any{"displayName": "Ada"},
				"body": map[string]any{
					"type":    "doc",
					"version": 1,
					"content": []any{map[string]any{
						"type":    "paragraph",
						"content": []any{map[string]any{"type": "text", "text": "hello"}},
					}},
				},
				"created": "2024-01-01T00:00:00.000+0000",
			}},
		})
	})

	result, err := c.ListComments(t.Context(), "PROJ-1", 10, 0)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if result.Total != 12 || len(result.Comments) != 1 {
		t.Fatalf("ListComments() = %+v", result)
	}
	if got := result.Comments[0].BodyMarkdown(); got != "hello" {
		t.Errorf("BodyMarkdown() = %q, want %q", got, "hello")
	}
	if result.Comments[0].Author == nil || result.Comments[0].Author.DisplayName != "Ada" {
		t.Errorf("unexpected author: %+v", result.Comments[0].Author)
	}
}

func TestGetMyself(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/myself" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accountId":    "acc-1",
			"displayName":  "Ada Lovelace",
			"emailAddress": "ada@example.com",
		})
	})

	user, err := c.GetMyself(t.Context())
	if err != nil {
		t.Fatalf("GetMyself() error = %v", err)
	}
	if user.AccountID != "acc-1" || user.DisplayName != "Ada Lovelace" {
		t.Errorf("GetMyself() = %+v", user)
	}
}

func TestSearchUsers(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/user/search" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "ada" {
			t.Fatalf("unexpected query: %q", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"accountId": "acc-1", "displayName": "Ada"},
			{"accountId": "acc-2", "displayName": "Adam"},
		})
	})

	users, err := c.SearchUsers(t.Context(), "ada", 0, 0)
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if len(users) != 2 || users[0].AccountID != "acc-1" || users[1].AccountID != "acc-2" {
		t.Errorf("SearchUsers() = %+v", users)
	}
}

func TestAssignIssue(t *testing.T) {
	tests := []struct {
		name      string
		accountID *string
		want      any
	}{
		{name: "assign", accountID: new("acc-1"), want: "acc-1"},
		{name: "unassign", accountID: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/rest/api/3/issue/PROJ-1/assignee" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if _, ok := body["accountId"]; !ok {
					t.Fatalf("missing accountId key in body: %v", body)
				}
				if body["accountId"] != tt.want {
					t.Fatalf("accountId = %v, want %v", body["accountId"], tt.want)
				}
				w.WriteHeader(http.StatusNoContent)
			})

			if err := c.AssignIssue(t.Context(), "PROJ-1", tt.accountID); err != nil {
				t.Fatalf("AssignIssue() error = %v", err)
			}
		})
	}
}

func TestGetIssue_ParsesRichFields(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key": "PROJ-1",
			"fields": map[string]any{
				"summary":  "Rich issue",
				"labels":   []string{"backend", "urgent"},
				"priority": map[string]any{"id": "2", "name": "High"},
				"duedate":  "2024-06-01",
				"parent":   map[string]any{"key": "PROJ-9"},
				"subtasks": []map[string]any{{"key": "PROJ-2"}},
				"components": []map[string]any{
					{"name": "api"},
				},
				"attachment": []map[string]any{{
					"id":       "42",
					"filename": "log.txt",
					"mimeType": "text/plain",
					"size":     float64(11),
				}},
				"issuelinks": []map[string]any{{
					"type":         map[string]any{"name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
					"outwardIssue": map[string]any{"key": "PROJ-3", "fields": map[string]any{"summary": "Other"}},
				}},
			},
		})
	})

	issue, err := c.GetIssue(t.Context(), "PROJ-1", nil)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	f := issue.Fields
	if len(f.Labels) != 2 || f.Labels[0] != "backend" {
		t.Errorf("labels = %v", f.Labels)
	}
	if f.Priority == nil || f.Priority.Name != "High" {
		t.Errorf("priority = %+v", f.Priority)
	}
	if f.DueDate != "2024-06-01" {
		t.Errorf("duedate = %q", f.DueDate)
	}
	if f.Parent == nil || f.Parent.Key != "PROJ-9" {
		t.Errorf("parent = %+v", f.Parent)
	}
	if len(f.Subtasks) != 1 || f.Subtasks[0].Key != "PROJ-2" {
		t.Errorf("subtasks = %+v", f.Subtasks)
	}
	if len(f.Components) != 1 || f.Components[0].Name != "api" {
		t.Errorf("components = %+v", f.Components)
	}
	if len(f.Attachments) != 1 || f.Attachments[0].ID != "42" || f.Attachments[0].Filename != "log.txt" {
		t.Errorf("attachments = %+v", f.Attachments)
	}
	if len(f.IssueLinks) != 1 || f.IssueLinks[0].OutwardIssue == nil || f.IssueLinks[0].OutwardIssue.Key != "PROJ-3" {
		t.Errorf("issuelinks = %+v", f.IssueLinks)
	}
}
