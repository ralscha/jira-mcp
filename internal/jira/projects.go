package jira

import (
	"context"
	"net/url"
	"strconv"
)

// ListProjects returns a page of projects visible to the authenticated
// user.
func (c *Client) ListProjects(ctx context.Context, startAt, maxResults int) (*ProjectSearchResult, error) {
	query := url.Values{}
	query.Set("startAt", strconv.Itoa(startAt))
	query.Set("maxResults", strconv.Itoa(maxResults))

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
