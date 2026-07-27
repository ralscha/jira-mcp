package jira

import (
	"context"
	"net/url"
	"strconv"
)

// ListIssueLinkTypes returns the issue link types configured on the site,
// e.g. "Blocks" or "Relates".
func (c *Client) ListIssueLinkTypes(ctx context.Context) ([]IssueLinkType, error) {
	var result struct {
		IssueLinkTypes []IssueLinkType `json:"issueLinkTypes"`
	}
	if err := c.doJSON(ctx, "GET", "rest/api/3/issueLinkType", nil, nil, &result); err != nil {
		return nil, err
	}
	return result.IssueLinkTypes, nil
}

// LinkIssues creates a link of the given type from inwardKey to outwardKey.
// For a link type named "Blocks", the outward issue blocks the inward one:
// outwardKey "blocks" inwardKey.
func (c *Client) LinkIssues(ctx context.Context, linkType, inwardKey, outwardKey, comment string) error {
	body := map[string]any{
		"type":         map[string]any{"name": linkType},
		"inwardIssue":  map[string]any{"key": inwardKey},
		"outwardIssue": map[string]any{"key": outwardKey},
	}
	if comment != "" {
		body["comment"] = map[string]any{"body": markdownToADF(comment)}
	}
	return c.doJSON(ctx, "POST", "rest/api/3/issueLink", nil, body, nil)
}

// Worklog is a work log entry on an issue. Comment is an ADF value.
type Worklog struct {
	ID               string `json:"id,omitempty"`
	Author           *User  `json:"author,omitempty"`
	Comment          any    `json:"comment,omitempty"`
	Started          string `json:"started,omitempty"`
	TimeSpent        string `json:"timeSpent,omitempty"`
	TimeSpentSeconds int64  `json:"timeSpentSeconds,omitempty"`
	Created          string `json:"created,omitempty"`
}

// CommentMarkdown extracts the Markdown rendering of a worklog's ADF
// comment, or "" if it has none.
func (w *Worklog) CommentMarkdown() string {
	if w.Comment == nil {
		return ""
	}
	return adfToMarkdown(w.Comment)
}

// WorklogResult is one page of GET /rest/api/3/issue/{key}/worklog.
type WorklogResult struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}

// ListWorklogs returns one page of an issue's work log entries.
func (c *Client) ListWorklogs(ctx context.Context, keyOrID string, startAt, maxResults int) (*WorklogResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	query := url.Values{}
	query.Set("startAt", strconv.Itoa(startAt))
	query.Set("maxResults", strconv.Itoa(maxResults))

	var result WorklogResult
	path := "rest/api/3/issue/" + url.PathEscape(keyOrID) + "/worklog"
	if err := c.doJSON(ctx, "GET", path, query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AddWorklogInput describes a new work log entry.
type AddWorklogInput struct {
	TimeSpent string // Jira duration, e.g. "3h 30m"
	Comment   string // Markdown; converted to ADF
	Started   string // ISO 8601 with offset, e.g. 2024-01-02T10:00:00.000+0000
}

// AddWorklog logs work against an issue.
func (c *Client) AddWorklog(ctx context.Context, keyOrID string, in AddWorklogInput) (*Worklog, error) {
	body := map[string]any{"timeSpent": in.TimeSpent}
	if in.Comment != "" {
		body["comment"] = markdownToADF(in.Comment)
	}
	if in.Started != "" {
		body["started"] = in.Started
	}

	var created Worklog
	path := "rest/api/3/issue/" + url.PathEscape(keyOrID) + "/worklog"
	if err := c.doJSON(ctx, "POST", path, nil, body, &created); err != nil {
		return nil, err
	}
	return &created, nil
}
