package jira

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

// IssueFields holds the subset of Jira issue fields used by jira-mcp.
// Description is an Atlassian Document Format (ADF) value; use
// adfToPlainText/plainTextToADF to convert to/from plain strings.
type IssueFields struct {
	Summary     string        `json:"summary,omitempty"`
	Description any           `json:"description,omitempty"`
	Status      *StatusRef    `json:"status,omitempty"`
	IssueType   *IssueTypeRef `json:"issuetype,omitempty"`
	Project     *ProjectRef   `json:"project,omitempty"`
	Assignee    *User         `json:"assignee,omitempty"`
	Reporter    *User         `json:"reporter,omitempty"`
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

// Attachment is Jira attachment metadata as returned by the attachment
// endpoints.
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	Content  string `json:"content"`
}
