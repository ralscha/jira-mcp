// Package mcpserver builds the jira-mcp MCP server, registering Jira tools
// against the official MCP Go SDK. Read tools are always registered; write
// tools are only registered when the server is running in readwrite mode.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jira-mcp/internal/jira"
)

// readOnlyHint is shared by all read-only tool registrations.
var readOnlyHint = &mcp.ToolAnnotations{ReadOnlyHint: true}

// IssueSummary is a flattened, human-readable view of a Jira issue.
type IssueSummary struct {
	Key         string `json:"key" jsonschema:"the issue key, e.g. PROJ-123"`
	Summary     string `json:"summary,omitempty" jsonschema:"the issue summary/title"`
	Status      string `json:"status,omitempty" jsonschema:"the current workflow status name"`
	IssueType   string `json:"issue_type,omitempty" jsonschema:"the issue type name, e.g. Bug, Task"`
	Project     string `json:"project,omitempty" jsonschema:"the project key"`
	Assignee    string `json:"assignee,omitempty" jsonschema:"the assignee's display name, if assigned"`
	Reporter    string `json:"reporter,omitempty" jsonschema:"the reporter's display name"`
	Description string `json:"description,omitempty" jsonschema:"the issue description as plain text"`
	Created     string `json:"created,omitempty" jsonschema:"creation timestamp"`
	Updated     string `json:"updated,omitempty" jsonschema:"last update timestamp"`
}

func issueToSummary(issue *jira.Issue) IssueSummary {
	s := IssueSummary{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: issue.Fields.DescriptionPlainText(),
		Created:     issue.Fields.Created,
		Updated:     issue.Fields.Updated,
	}
	if issue.Fields.Status != nil {
		s.Status = issue.Fields.Status.Name
	}
	if issue.Fields.IssueType != nil {
		s.IssueType = issue.Fields.IssueType.Name
	}
	if issue.Fields.Project != nil {
		s.Project = issue.Fields.Project.Key
	}
	if issue.Fields.Assignee != nil {
		s.Assignee = issue.Fields.Assignee.DisplayName
	}
	if issue.Fields.Reporter != nil {
		s.Reporter = issue.Fields.Reporter.DisplayName
	}
	return s
}

// GetIssueInput is the input for the jira_get_issue tool.
type GetIssueInput struct {
	IssueKey string   `json:"issue_key" jsonschema:"the Jira issue key or id, e.g. PROJ-123"`
	Fields   []string `json:"fields,omitempty" jsonschema:"optional list of Jira field names to fetch; defaults to a standard set if omitted"`
}

func getIssue(client *jira.Client) mcp.ToolHandlerFor[GetIssueInput, IssueSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetIssueInput) (*mcp.CallToolResult, IssueSummary, error) {
		issue, err := client.GetIssue(ctx, in.IssueKey, in.Fields)
		if err != nil {
			return nil, IssueSummary{}, fmt.Errorf("get issue %s: %w", in.IssueKey, err)
		}
		return nil, issueToSummary(issue), nil
	}
}

// SearchIssuesInput is the input for the jira_search_issues tool.
type SearchIssuesInput struct {
	JQL        string   `json:"jql" jsonschema:"a JQL (Jira Query Language) query string, e.g. 'project = PROJ AND status = \"To Do\"'"`
	StartAt    int      `json:"start_at,omitempty" jsonschema:"pagination offset, defaults to 0"`
	MaxResults int      `json:"max_results,omitempty" jsonschema:"maximum number of results to return, defaults to 50"`
	Fields     []string `json:"fields,omitempty" jsonschema:"optional list of Jira field names to fetch per issue"`
}

// SearchIssuesOutput is the output for the jira_search_issues tool.
type SearchIssuesOutput struct {
	Total      int            `json:"total" jsonschema:"total number of matching issues"`
	StartAt    int            `json:"start_at" jsonschema:"pagination offset of this page"`
	MaxResults int            `json:"max_results" jsonschema:"maximum results requested for this page"`
	Issues     []IssueSummary `json:"issues" jsonschema:"the matching issues in this page"`
}

func searchIssues(client *jira.Client) mcp.ToolHandlerFor[SearchIssuesInput, SearchIssuesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SearchIssuesInput) (*mcp.CallToolResult, SearchIssuesOutput, error) {
		maxResults := in.MaxResults
		if maxResults <= 0 {
			maxResults = 50
		}
		result, err := client.SearchIssues(ctx, in.JQL, in.StartAt, maxResults, in.Fields)
		if err != nil {
			return nil, SearchIssuesOutput{}, fmt.Errorf("search issues: %w", err)
		}
		out := SearchIssuesOutput{
			Total:      result.Total,
			StartAt:    result.StartAt,
			MaxResults: result.MaxResults,
			Issues:     make([]IssueSummary, len(result.Issues)),
		}
		for i := range result.Issues {
			out.Issues[i] = issueToSummary(&result.Issues[i])
		}
		return nil, out, nil
	}
}

// ProjectSummary is a flattened view of a Jira project.
type ProjectSummary struct {
	Key            string `json:"key" jsonschema:"the project key"`
	Name           string `json:"name,omitempty" jsonschema:"the project display name"`
	ProjectTypeKey string `json:"project_type_key,omitempty" jsonschema:"the project type, e.g. software, business"`
}

func projectToSummary(p *jira.Project) ProjectSummary {
	return ProjectSummary{Key: p.Key, Name: p.Name, ProjectTypeKey: p.ProjectTypeKey}
}

// ListProjectsInput is the input for the jira_list_projects tool.
type ListProjectsInput struct {
	StartAt    int `json:"start_at,omitempty" jsonschema:"pagination offset, defaults to 0"`
	MaxResults int `json:"max_results,omitempty" jsonschema:"maximum number of results to return, defaults to 50"`
}

// ListProjectsOutput is the output for the jira_list_projects tool.
type ListProjectsOutput struct {
	Total    int              `json:"total" jsonschema:"total number of visible projects"`
	Projects []ProjectSummary `json:"projects" jsonschema:"the projects in this page"`
}

func listProjects(client *jira.Client) mcp.ToolHandlerFor[ListProjectsInput, ListProjectsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListProjectsInput) (*mcp.CallToolResult, ListProjectsOutput, error) {
		maxResults := in.MaxResults
		if maxResults <= 0 {
			maxResults = 50
		}
		result, err := client.ListProjects(ctx, in.StartAt, maxResults)
		if err != nil {
			return nil, ListProjectsOutput{}, fmt.Errorf("list projects: %w", err)
		}
		out := ListProjectsOutput{
			Total:    result.Total,
			Projects: make([]ProjectSummary, len(result.Values)),
		}
		for i := range result.Values {
			out.Projects[i] = projectToSummary(&result.Values[i])
		}
		return nil, out, nil
	}
}

// GetProjectInput is the input for the jira_get_project tool.
type GetProjectInput struct {
	ProjectKey string `json:"project_key" jsonschema:"the Jira project key or id"`
}

func getProject(client *jira.Client) mcp.ToolHandlerFor[GetProjectInput, ProjectSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetProjectInput) (*mcp.CallToolResult, ProjectSummary, error) {
		project, err := client.GetProject(ctx, in.ProjectKey)
		if err != nil {
			return nil, ProjectSummary{}, fmt.Errorf("get project %s: %w", in.ProjectKey, err)
		}
		return nil, projectToSummary(project), nil
	}
}

// GetTransitionsInput is the input for the jira_get_transitions tool.
type GetTransitionsInput struct {
	IssueKey string `json:"issue_key" jsonschema:"the Jira issue key or id, e.g. PROJ-123"`
}

// TransitionSummary describes an available workflow transition.
type TransitionSummary struct {
	ID       string `json:"id" jsonschema:"the transition id, used with jira_transition_issue"`
	Name     string `json:"name" jsonschema:"the transition's human-readable name, e.g. 'Start Progress'"`
	ToStatus string `json:"to_status,omitempty" jsonschema:"the workflow status this transition leads to"`
}

// GetTransitionsOutput is the output for the jira_get_transitions tool.
type GetTransitionsOutput struct {
	Transitions []TransitionSummary `json:"transitions" jsonschema:"the transitions currently available for the issue"`
}

func getTransitions(client *jira.Client) mcp.ToolHandlerFor[GetTransitionsInput, GetTransitionsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetTransitionsInput) (*mcp.CallToolResult, GetTransitionsOutput, error) {
		transitions, err := client.GetTransitions(ctx, in.IssueKey)
		if err != nil {
			return nil, GetTransitionsOutput{}, fmt.Errorf("get transitions for %s: %w", in.IssueKey, err)
		}
		out := GetTransitionsOutput{Transitions: make([]TransitionSummary, len(transitions))}
		for i, tr := range transitions {
			ts := TransitionSummary{ID: tr.ID, Name: tr.Name}
			if tr.To != nil {
				ts.ToStatus = tr.To.Name
			}
			out.Transitions[i] = ts
		}
		return nil, out, nil
	}
}

// DownloadAttachmentInput is the input for the jira_download_attachment tool.
type DownloadAttachmentInput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the Jira attachment id"`
}

// DownloadAttachmentOutput is the output for the jira_download_attachment tool.
type DownloadAttachmentOutput struct {
	Filename   string `json:"filename" jsonschema:"the attachment's original filename"`
	MimeType   string `json:"mime_type,omitempty" jsonschema:"the attachment's MIME type"`
	Size       int64  `json:"size,omitempty" jsonschema:"the attachment size in bytes"`
	DataBase64 string `json:"data_base64" jsonschema:"the attachment content, base64-encoded"`
}

func downloadAttachment(client *jira.Client) mcp.ToolHandlerFor[DownloadAttachmentInput, DownloadAttachmentOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DownloadAttachmentInput) (*mcp.CallToolResult, DownloadAttachmentOutput, error) {
		attachment, err := client.DownloadAttachment(ctx, in.AttachmentID)
		if err != nil {
			return nil, DownloadAttachmentOutput{}, fmt.Errorf("download attachment %s: %w", in.AttachmentID, err)
		}
		return nil, DownloadAttachmentOutput{
			Filename:   attachment.Filename,
			MimeType:   attachment.MimeType,
			Size:       attachment.Size,
			DataBase64: attachment.DataBase64,
		}, nil
	}
}

// registerReadTools registers all read-only Jira tools on the server.
func registerReadTools(s *mcp.Server, client *jira.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_get_issue",
		Description: "Get a single Jira issue by key or id.",
		Annotations: readOnlyHint,
	}, getIssue(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_search_issues",
		Description: "Search Jira issues using a JQL query, with pagination.",
		Annotations: readOnlyHint,
	}, searchIssues(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_list_projects",
		Description: "List Jira projects visible to the authenticated user, with pagination.",
		Annotations: readOnlyHint,
	}, listProjects(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_get_project",
		Description: "Get a single Jira project by key or id.",
		Annotations: readOnlyHint,
	}, getProject(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_get_transitions",
		Description: "List the workflow transitions currently available for a Jira issue.",
		Annotations: readOnlyHint,
	}, getTransitions(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_download_attachment",
		Description: "Download a Jira attachment's content by id, base64-encoded.",
		Annotations: readOnlyHint,
	}, downloadAttachment(client))
}
