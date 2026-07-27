package jira

import (
	"context"
	"net/url"
	"strconv"
)

// ProjectSearchOptions filters and paginates a project search.
type ProjectSearchOptions struct {
	Query      string // matches project key or name
	TypeKey    string // e.g. software, business, service_desk
	OrderBy    string // e.g. key, name, lastIssueUpdatedTime
	StartAt    int
	MaxResults int
}

// ListProjects returns a page of projects visible to the authenticated
// user, filtered by opts.
func (c *Client) ListProjects(ctx context.Context, opts ProjectSearchOptions) (*ProjectSearchResult, error) {
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}
	query := url.Values{}
	query.Set("startAt", strconv.Itoa(opts.StartAt))
	query.Set("maxResults", strconv.Itoa(opts.MaxResults))
	if opts.Query != "" {
		query.Set("query", opts.Query)
	}
	if opts.TypeKey != "" {
		query.Set("typeKey", opts.TypeKey)
	}
	if opts.OrderBy != "" {
		query.Set("orderBy", opts.OrderBy)
	}

	var result ProjectSearchResult
	if err := c.doJSON(ctx, "GET", "rest/api/3/project/search", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProject fetches a single project by key or id.
func (c *Client) GetProject(ctx context.Context, keyOrID string) (*Project, error) {
	var project Project
	if err := c.doJSON(ctx, "GET", "rest/api/3/project/"+url.PathEscape(keyOrID), nil, nil, &project); err != nil {
		return nil, err
	}
	return &project, nil
}
