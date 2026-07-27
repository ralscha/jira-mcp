package jira

import (
	"context"
	"net/url"
	"strconv"
)

// ListComments returns one page of an issue's comments, oldest first.
func (c *Client) ListComments(ctx context.Context, keyOrID string, startAt, maxResults int) (*CommentsResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	query := url.Values{}
	query.Set("startAt", strconv.Itoa(startAt))
	query.Set("maxResults", strconv.Itoa(maxResults))

	var result CommentsResult
	path := "rest/api/3/issue/" + url.PathEscape(keyOrID) + "/comment"
	if err := c.doJSON(ctx, "GET", path, query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
