package jira

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Field is a Jira field definition as returned by GET /rest/api/3/field.
type Field struct {
	ID     string `json:"id"`
	Key    string `json:"key,omitempty"`
	Name   string `json:"name,omitempty"`
	Custom bool   `json:"custom"`
	Schema *struct {
		Type   string `json:"type,omitempty"`
		Items  string `json:"items,omitempty"`
		Custom string `json:"custom,omitempty"`
	} `json:"schema,omitempty"`
}

// ListFields returns all fields visible to the authenticated user. When
// query is non-empty, only fields whose id or name contains it
// (case-insensitively) are returned.
func (c *Client) ListFields(ctx context.Context, query string) ([]Field, error) {
	var fields []Field
	if err := c.doJSON(ctx, "GET", "rest/api/3/field", nil, nil, &fields); err != nil {
		return nil, err
	}
	if query == "" {
		return fields, nil
	}
	needle := strings.ToLower(query)
	matched := make([]Field, 0, len(fields))
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f.Name), needle) || strings.Contains(strings.ToLower(f.ID), needle) {
			matched = append(matched, f)
		}
	}
	return matched, nil
}

// CreateMetaIssueType is an issue type available when creating issues in a
// project.
type CreateMetaIssueType struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Subtask     bool   `json:"subtask"`
}

type createMetaIssueTypesResult struct {
	StartAt    int                   `json:"startAt"`
	Total      int                   `json:"total"`
	IssueTypes []CreateMetaIssueType `json:"issueTypes"`
}

// ListCreateIssueTypes returns the issue types that can be created in a
// project.
func (c *Client) ListCreateIssueTypes(ctx context.Context, projectKeyOrID string) ([]CreateMetaIssueType, error) {
	path := "rest/api/3/issue/createmeta/" + url.PathEscape(projectKeyOrID) + "/issuetypes"
	return collectCreateMetaPages("issue type", func(startAt int) (int, int, []CreateMetaIssueType, error) {
		query := url.Values{}
		query.Set("startAt", strconv.Itoa(startAt))
		query.Set("maxResults", strconv.Itoa(200))

		var page createMetaIssueTypesResult
		if err := c.doJSON(ctx, "GET", path, query, nil, &page); err != nil {
			return 0, 0, nil, err
		}
		return page.StartAt, page.Total, page.IssueTypes, nil
	})
}

// CreateMetaField describes a field on the create screen for a given
// project and issue type.
type CreateMetaField struct {
	FieldID  string `json:"fieldId"`
	Name     string `json:"name,omitempty"`
	Required bool   `json:"required"`
	Schema   *struct {
		Type   string `json:"type,omitempty"`
		Items  string `json:"items,omitempty"`
		Custom string `json:"custom,omitempty"`
	} `json:"schema,omitempty"`
	AllowedValues []map[string]any `json:"allowedValues,omitempty"`
}

type createMetaFieldsResult struct {
	StartAt int               `json:"startAt"`
	Total   int               `json:"total"`
	Fields  []CreateMetaField `json:"fields"`
}

// ListCreateFields returns the fields available on the create screen for a
// project and issue type. issueTypeIDOrName is resolved against the
// project's issue types when it is not an id.
func (c *Client) ListCreateFields(ctx context.Context, projectKeyOrID, issueTypeIDOrName string) ([]CreateMetaField, error) {
	issueTypeID, err := c.resolveIssueTypeID(ctx, projectKeyOrID, issueTypeIDOrName)
	if err != nil {
		return nil, err
	}

	path := "rest/api/3/issue/createmeta/" + url.PathEscape(projectKeyOrID) +
		"/issuetypes/" + url.PathEscape(issueTypeID)
	return collectCreateMetaPages("field", func(startAt int) (int, int, []CreateMetaField, error) {
		query := url.Values{}
		query.Set("startAt", strconv.Itoa(startAt))
		query.Set("maxResults", strconv.Itoa(200))

		var page createMetaFieldsResult
		if err := c.doJSON(ctx, "GET", path, query, nil, &page); err != nil {
			return 0, 0, nil, err
		}
		return page.StartAt, page.Total, page.Fields, nil
	})
}

func collectCreateMetaPages[T any](kind string, fetch func(int) (int, int, []T, error)) ([]T, error) {
	var all []T
	for startAt := 0; ; {
		pageStart, total, values, err := fetch(startAt)
		if err != nil {
			return nil, err
		}
		all = append(all, values...)
		next := pageStart + len(values)
		if len(values) == 0 || next >= total {
			return all, nil
		}
		if next <= startAt {
			return nil, fmt.Errorf("jira: create %s pagination did not advance", kind)
		}
		startAt = next
	}
}

func (c *Client) resolveIssueTypeID(ctx context.Context, projectKeyOrID, issueTypeIDOrName string) (string, error) {
	if _, err := strconv.Atoi(issueTypeIDOrName); err == nil {
		return issueTypeIDOrName, nil
	}
	issueTypes, err := c.ListCreateIssueTypes(ctx, projectKeyOrID)
	if err != nil {
		return "", err
	}
	for _, it := range issueTypes {
		if strings.EqualFold(it.Name, issueTypeIDOrName) {
			return it.ID, nil
		}
	}
	return "", &NotFoundError{What: "issue type " + issueTypeIDOrName + " in project " + projectKeyOrID}
}

// NotFoundError reports that a name could not be resolved locally, without
// a corresponding Jira HTTP status.
type NotFoundError struct {
	What string
}

func (e *NotFoundError) Error() string {
	return "jira: not found: " + e.What
}
