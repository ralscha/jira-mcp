package jira

import (
	"encoding/json"
	"fmt"
)

// User is a minimal Jira Cloud user reference (assignee, reporter, author).
type User struct {
	AccountID    string `json:"accountId,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
}

// StatusRef references an issue's workflow status.
type StatusRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// IssueTypeRef references an issue type (e.g. Task, Bug, Story).
type IssueTypeRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// ProjectRef references a project by key/id/name.
type ProjectRef struct {
	ID   string `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

// NamedRef is a generic id/name reference, used for priority, resolution,
// components and versions.
type NamedRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// IssueRefFields is the small field subset Jira embeds in parent, subtask
// and issue link references.
type IssueRefFields struct {
	Summary   string        `json:"summary,omitempty"`
	Status    *StatusRef    `json:"status,omitempty"`
	IssueType *IssueTypeRef `json:"issuetype,omitempty"`
}

// IssueRef is a lightweight reference to another issue.
type IssueRef struct {
	ID     string          `json:"id,omitempty"`
	Key    string          `json:"key,omitempty"`
	Fields *IssueRefFields `json:"fields,omitempty"`
}

// IssueLinkType describes the semantics of an issue link, e.g. name
// "Blocks" with inward "is blocked by" and outward "blocks".
type IssueLinkType struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Inward  string `json:"inward,omitempty"`
	Outward string `json:"outward,omitempty"`
}

// IssueLink is a link between two issues. Exactly one of InwardIssue and
// OutwardIssue is set on links returned as part of an issue's fields.
type IssueLink struct {
	ID           string         `json:"id,omitempty"`
	Type         *IssueLinkType `json:"type,omitempty"`
	InwardIssue  *IssueRef      `json:"inwardIssue,omitempty"`
	OutwardIssue *IssueRef      `json:"outwardIssue,omitempty"`
}

// IssueFields holds the subset of Jira issue fields used by jira-mcp.
// Description is an Atlassian Document Format (ADF) value; use
// adfToMarkdown/markdownToADF to convert to/from plain strings.
type IssueFields struct {
	Summary     string        `json:"summary,omitempty"`
	Description any           `json:"description,omitempty"`
	Status      *StatusRef    `json:"status,omitempty"`
	IssueType   *IssueTypeRef `json:"issuetype,omitempty"`
	Project     *ProjectRef   `json:"project,omitempty"`
	Assignee    *User         `json:"assignee,omitempty"`
	Reporter    *User         `json:"reporter,omitempty"`
	Priority    *NamedRef     `json:"priority,omitempty"`
	Resolution  *NamedRef     `json:"resolution,omitempty"`
	Labels      []string      `json:"labels,omitempty"`
	Components  []NamedRef    `json:"components,omitempty"`
	FixVersions []NamedRef    `json:"fixVersions,omitempty"`
	Parent      *IssueRef     `json:"parent,omitempty"`
	Subtasks    []IssueRef    `json:"subtasks,omitempty"`
	IssueLinks  []IssueLink   `json:"issuelinks,omitempty"`
	Attachments []Attachment  `json:"attachment,omitempty"`
	DueDate     string        `json:"duedate,omitempty"`
	Created     string        `json:"created,omitempty"`
	Updated     string        `json:"updated,omitempty"`
}

// Issue is a Jira issue as returned by the get/search/create endpoints.
type Issue struct {
	ID     string      `json:"id,omitempty"`
	Key    string      `json:"key,omitempty"`
	Self   string      `json:"self,omitempty"`
	Fields IssueFields `json:"fields"`
}

// SearchResult is one token-paginated page returned by Jira's enhanced issue
// search API.
type SearchResult struct {
	IsLast        bool    `json:"isLast"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
	Issues        []Issue `json:"issues"`
}

// Project is a Jira project as returned by the project endpoints.
type Project struct {
	ID             string `json:"id,omitempty"`
	Key            string `json:"key,omitempty"`
	Name           string `json:"name,omitempty"`
	ProjectTypeKey string `json:"projectTypeKey,omitempty"`
}

// ProjectSearchResult is the response body of GET /rest/api/3/project/search.
type ProjectSearchResult struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	IsLast     bool      `json:"isLast"`
	Values     []Project `json:"values"`
}

// Transition is a workflow transition available on an issue.
type Transition struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	To   *StatusRef `json:"to,omitempty"`
}

// TransitionsResult is the response body of GET .../issue/{key}/transitions.
type TransitionsResult struct {
	Transitions []Transition `json:"transitions"`
}

// Comment is a Jira issue comment. Body is an ADF value.
type Comment struct {
	ID      string `json:"id,omitempty"`
	Body    any    `json:"body,omitempty"`
	Author  *User  `json:"author,omitempty"`
	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
}

// CommentsResult is one page of the response body of
// GET /rest/api/3/issue/{key}/comment.
type CommentsResult struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Comments   []Comment `json:"comments"`
}

// Attachment is Jira attachment metadata as returned by the attachment
// endpoints and by an issue's "attachment" field.
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	Content  string `json:"content"`
	Created  string `json:"created,omitempty"`
	Author   *User  `json:"author,omitempty"`
}

// UnmarshalJSON accepts both representations Jira uses for attachment IDs:
// numbers in attachment metadata and strings in issue/upload responses.
func (a *Attachment) UnmarshalJSON(data []byte) error {
	type attachment Attachment
	var decoded struct {
		ID json.RawMessage `json:"id"`
		*attachment
	}

	decoded.attachment = (*attachment)(a)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	if len(decoded.ID) == 0 || string(decoded.ID) == "null" {
		a.ID = ""
		return nil
	}
	if decoded.ID[0] == '"' {
		if err := json.Unmarshal(decoded.ID, &a.ID); err != nil {
			return fmt.Errorf("jira attachment id: %w", err)
		}
		return nil
	}

	var id json.Number
	if err := json.Unmarshal(decoded.ID, &id); err != nil {
		return fmt.Errorf("jira attachment id: %w", err)
	}
	a.ID = id.String()
	return nil
}
