package jira

import (
	"context"
	"fmt"
	"maps"
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
	"priority",
	"resolution",
	"labels",
	"components",
	"fixVersions",
	"parent",
	"subtasks",
	"issuelinks",
	"attachment",
	"duedate",
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

// CreateIssueInput describes the fields for a new issue. Empty values are
// omitted from the request.
type CreateIssueInput struct {
	ProjectKey        string
	IssueType         string
	Summary           string
	Description       string // Markdown; converted to ADF
	ParentKey         string // parent issue or epic key, for subtasks
	AssigneeAccountID string
	Priority          string // priority name, e.g. "High"
	Labels            []string
	Components        []string // component names
	FixVersions       []string // fix version names
	DueDate           string   // YYYY-MM-DD
	Fields            map[string]any
}

// CreateIssue creates a new issue and returns the created issue's key/id.
func (c *Client) CreateIssue(ctx context.Context, in CreateIssueInput) (*Issue, error) {
	fields := maps.Clone(in.Fields)
	if fields == nil {
		fields = make(map[string]any)
	}
	// Dedicated arguments take precedence over the raw fields escape hatch.
	fields["project"] = map[string]any{"key": in.ProjectKey}
	fields["issuetype"] = map[string]any{"name": in.IssueType}
	fields["summary"] = in.Summary
	if in.Description != "" {
		fields["description"] = markdownToADF(in.Description)
	}
	if in.ParentKey != "" {
		fields["parent"] = map[string]any{"key": in.ParentKey}
	}
	if in.AssigneeAccountID != "" {
		fields["assignee"] = map[string]any{"accountId": in.AssigneeAccountID}
	}
	if in.Priority != "" {
		fields["priority"] = map[string]any{"name": in.Priority}
	}
	if len(in.Labels) > 0 {
		fields["labels"] = in.Labels
	}
	if len(in.Components) > 0 {
		fields["components"] = namedRefs(in.Components)
	}
	if len(in.FixVersions) > 0 {
		fields["fixVersions"] = namedRefs(in.FixVersions)
	}
	if in.DueDate != "" {
		fields["duedate"] = in.DueDate
	}
	body := map[string]any{"fields": fields}

	var created Issue
	if err := c.doJSON(ctx, "POST", "rest/api/3/issue", nil, body, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// UpdateIssueInput describes the fields to change on an existing issue. Nil
// fields are left unmodified. A non-nil pointer to an empty string clears
// the corresponding field where Jira allows it.
type UpdateIssueInput struct {
	Summary           *string
	Description       *string // Markdown; converted to ADF
	AssigneeAccountID *string // empty string unassigns
	Priority          *string
	Labels            *[]string
	Components        *[]string
	FixVersions       *[]string
	DueDate           *string // YYYY-MM-DD; empty string clears the due date
	Fields            map[string]any
}

// UpdateIssue updates the given fields on an existing issue.
func (c *Client) UpdateIssue(ctx context.Context, keyOrID string, in UpdateIssueInput) error {
	fields := maps.Clone(in.Fields)
	if fields == nil {
		fields = make(map[string]any)
	}
	if in.Summary != nil {
		fields["summary"] = *in.Summary
	}
	if in.Description != nil {
		if *in.Description == "" {
			fields["description"] = nil
		} else {
			fields["description"] = markdownToADF(*in.Description)
		}
	}
	if in.AssigneeAccountID != nil {
		if *in.AssigneeAccountID == "" {
			fields["assignee"] = nil
		} else {
			fields["assignee"] = map[string]any{"accountId": *in.AssigneeAccountID}
		}
	}
	if in.Priority != nil {
		fields["priority"] = map[string]any{"name": *in.Priority}
	}
	if in.Labels != nil {
		labels := *in.Labels
		if labels == nil {
			labels = []string{}
		}
		fields["labels"] = labels
	}
	if in.Components != nil {
		fields["components"] = namedRefs(*in.Components)
	}
	if in.FixVersions != nil {
		fields["fixVersions"] = namedRefs(*in.FixVersions)
	}
	if in.DueDate != nil {
		if *in.DueDate == "" {
			fields["duedate"] = nil
		} else {
			fields["duedate"] = *in.DueDate
		}
	}
	if len(fields) == 0 {
		return fmt.Errorf("jira: UpdateIssue requires at least one field to update")
	}
	body := map[string]any{"fields": fields}
	return c.doJSON(ctx, "PUT", "rest/api/3/issue/"+url.PathEscape(keyOrID), nil, body, nil)
}

// namedRefs converts a list of names into the {"name": ...} objects Jira
// expects for fields such as components and fixVersions.
func namedRefs(names []string) []map[string]any {
	refs := make([]map[string]any, 0, len(names))
	for _, name := range names {
		refs = append(refs, map[string]any{"name": name})
	}
	return refs
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

// TransitionInput describes a workflow transition to execute, optionally
// setting fields (many workflows require a resolution) and adding a comment
// in the same request.
type TransitionInput struct {
	TransitionID string
	Resolution   string // resolution name, e.g. "Done"
	Comment      string // Markdown; converted to ADF
	Fields       map[string]any
}

// DoTransition executes a workflow transition (by id, as returned from
// GetTransitions) on an issue.
func (c *Client) DoTransition(ctx context.Context, keyOrID string, in TransitionInput) error {
	body := map[string]any{
		"transition": map[string]any{"id": in.TransitionID},
	}

	fields := map[string]any{}
	if in.Resolution != "" {
		fields["resolution"] = map[string]any{"name": in.Resolution}
	}
	maps.Copy(fields, in.Fields)
	if len(fields) > 0 {
		body["fields"] = fields
	}

	if in.Comment != "" {
		body["update"] = map[string]any{
			"comment": []any{
				map[string]any{"add": map[string]any{"body": markdownToADF(in.Comment)}},
			},
		}
	}

	return c.doJSON(ctx, "POST", "rest/api/3/issue/"+url.PathEscape(keyOrID)+"/transitions", nil, body, nil)
}

// AddComment adds a comment to an issue, converting Markdown to ADF.
func (c *Client) AddComment(ctx context.Context, keyOrID, text string) (*Comment, error) {
	body := map[string]any{
		"body": markdownToADF(text),
	}
	var comment Comment
	if err := c.doJSON(ctx, "POST", "rest/api/3/issue/"+url.PathEscape(keyOrID)+"/comment", nil, body, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// DescriptionMarkdown extracts the Markdown rendering of an issue's ADF
// description, or "" if it has none.
func (f *IssueFields) DescriptionMarkdown() string {
	if f.Description == nil {
		return ""
	}
	return adfToMarkdown(f.Description)
}

// BodyMarkdown extracts the Markdown rendering of a comment's ADF body,
// or "" if it has none.
func (c *Comment) BodyMarkdown() string {
	if c.Body == nil {
		return ""
	}
	return adfToMarkdown(c.Body)
}
