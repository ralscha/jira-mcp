package jira

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var defaultSearchFields = []string{
	"summary",
	"status",
	"issuetype",
	"project",
	"assignee",
	"reporter",
	"description",
	"created",
	"updated",
}

type searchRequest struct {
	JQL           string   `json:"jql"`
	Fields        []string `json:"fields,omitempty"`
	MaxResults    int      `json:"maxResults,omitempty"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

// GetIssue fetches a single issue by key or id. If fields is non-empty, only
// those fields are requested; otherwise Jira's default field set is used.
func (c *Client) GetIssue(ctx context.Context, keyOrID string, fields []string) (*Issue, error) {
	query := url.Values{}
	if len(fields) > 0 {
		query.Set("fields", strings.Join(fields, ","))
	}
	var issue Issue
	if err := c.doJSON(ctx, "GET", "rest/api/3/issue/"+url.PathEscape(keyOrID), query, nil, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// SearchIssues runs a JQL search, returning one token-paginated page of
// matching issues. Pass the previous result's NextPageToken to fetch the next
// page, or an empty string to start a new search.
func (c *Client) SearchIssues(ctx context.Context, jql, nextPageToken string, maxResults int, fields []string) (*SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}

	effectiveFields := fields
	if len(effectiveFields) == 0 {
		effectiveFields = append([]string(nil), defaultSearchFields...)
	}

	body := searchRequest{
		JQL:           jql,
		Fields:        effectiveFields,
		MaxResults:    maxResults,
		NextPageToken: nextPageToken,
	}

	var result SearchResult
	if err := c.doJSON(ctx, "POST", "rest/api/3/search/jql", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateIssueInput describes the fields for a new issue.
type CreateIssueInput struct {
	ProjectKey  string
	IssueType   string
	Summary     string
	Description string // plain text; converted to ADF
}

// CreateIssue creates a new issue and returns the created issue's key/id.
func (c *Client) CreateIssue(ctx context.Context, in CreateIssueInput) (*Issue, error) {
	fields := map[string]any{
		"project":   map[string]any{"key": in.ProjectKey},
		"issuetype": map[string]any{"name": in.IssueType},
		"summary":   in.Summary,
	}
	if in.Description != "" {
		fields["description"] = plainTextToADF(in.Description)
	}
	body := map[string]any{"fields": fields}

	var created Issue
	if err := c.doJSON(ctx, "POST", "rest/api/3/issue", nil, body, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// UpdateIssueInput describes the fields to change on an existing issue. Nil
// fields are left unmodified.
type UpdateIssueInput struct {
	Summary     *string
	Description *string // plain text; converted to ADF
}

// UpdateIssue updates the given fields on an existing issue.
func (c *Client) UpdateIssue(ctx context.Context, keyOrID string, in UpdateIssueInput) error {
	fields := map[string]any{}
	if in.Summary != nil {
		fields["summary"] = *in.Summary
	}
	if in.Description != nil {
		fields["description"] = plainTextToADF(*in.Description)
	}
	if len(fields) == 0 {
		return fmt.Errorf("jira: UpdateIssue requires at least one field to update")
	}
	body := map[string]any{"fields": fields}
	return c.doJSON(ctx, "PUT", "rest/api/3/issue/"+url.PathEscape(keyOrID), nil, body, nil)
}

// GetTransitions lists the workflow transitions currently available for an
// issue.
func (c *Client) GetTransitions(ctx context.Context, keyOrID string) ([]Transition, error) {
	var result TransitionsResult
	if err := c.doJSON(ctx, "GET", "rest/api/3/issue/"+url.PathEscape(keyOrID)+"/transitions", nil, nil, &result); err != nil {
		return nil, err
	}
	return result.Transitions, nil
}

// DoTransition executes a workflow transition (by id, as returned from
// GetTransitions) on an issue.
func (c *Client) DoTransition(ctx context.Context, keyOrID, transitionID string) error {
	body := map[string]any{
		"transition": map[string]any{"id": transitionID},
	}
	return c.doJSON(ctx, "POST", "rest/api/3/issue/"+url.PathEscape(keyOrID)+"/transitions", nil, body, nil)
}

// AddComment adds a plain-text comment to an issue.
func (c *Client) AddComment(ctx context.Context, keyOrID, text string) (*Comment, error) {
	body := map[string]any{
		"body": plainTextToADF(text),
	}
	var comment Comment
	if err := c.doJSON(ctx, "POST", "rest/api/3/issue/"+url.PathEscape(keyOrID)+"/comment", nil, body, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// DescriptionPlainText extracts the plain-text rendering of an issue's ADF
// description, or "" if it has none.
func (f *IssueFields) DescriptionPlainText() string {
	if f.Description == nil {
		return ""
	}
	return adfToPlainText(f.Description)
}

// BodyPlainText extracts the plain-text rendering of a comment's ADF body,
// or "" if it has none.
func (c *Comment) BodyPlainText() string {
	if c.Body == nil {
		return ""
	}
	return adfToPlainText(c.Body)
}

// unused import guard (strconv reserved for future pagination helpers).
