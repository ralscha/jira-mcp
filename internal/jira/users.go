package jira

import (
	"context"
	"net/url"
	"strconv"
)

// GetMyself returns the user the API token authenticates as.
func (c *Client) GetMyself(ctx context.Context) (*User, error) {
	var user User
	if err := c.doJSON(ctx, "GET", "rest/api/3/myself", nil, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// SearchUsers finds users whose display name or email matches query. It is
// the usual way to resolve a human-readable name to the accountId required
// by the assignee and reporter fields.
func (c *Client) SearchUsers(ctx context.Context, query string, startAt, maxResults int) ([]User, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	values := url.Values{}
	values.Set("query", query)
	values.Set("startAt", strconv.Itoa(startAt))
	values.Set("maxResults", strconv.Itoa(maxResults))

	var users []User
	if err := c.doJSON(ctx, "GET", "rest/api/3/user/search", values, nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// AssignIssue sets an issue's assignee. A nil accountID unassigns the
// issue; the special value "-1" assigns the project's default assignee.
func (c *Client) AssignIssue(ctx context.Context, keyOrID string, accountID *string) error {
	body := map[string]any{"accountId": nil}
	if accountID != nil {
		body["accountId"] = *accountID
	}
	return c.doJSON(ctx, "PUT", "rest/api/3/issue/"+url.PathEscape(keyOrID)+"/assignee", nil, body, nil)
}
