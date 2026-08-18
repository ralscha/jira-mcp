// Package mcpserver builds the jira-mcp MCP server, registering Jira tools
// against the official MCP Go SDK. Read tools are always registered; write
// tools are only registered when the server is running in readwrite mode.
package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jira-mcp/internal/jira"
)

// readOnlyHint is shared by all read-only tool registrations.
var readOnlyHint = &mcp.ToolAnnotations{ReadOnlyHint: true}

// AttachmentSummary is a flattened view of an attachment's metadata. The id
// is what jira_download_attachment expects.
type AttachmentSummary struct {
	ID       string `json:"id" jsonschema:"the attachment id, pass this to jira_download_attachment"`
	Filename string `json:"filename,omitempty" jsonschema:"the attachment's filename"`
	MimeType string `json:"mime_type,omitempty" jsonschema:"the attachment's MIME type"`
	Size     int64  `json:"size,omitempty" jsonschema:"the attachment size in bytes"`
	Created  string `json:"created,omitempty" jsonschema:"when the attachment was added"`
}

// IssueLinkSummary is a flattened view of a link between two issues.
type IssueLinkSummary struct {
	Type         string `json:"type" jsonschema:"the link relationship as seen from this issue, e.g. 'blocks' or 'is blocked by'"`
	IssueKey     string `json:"issue_key" jsonschema:"the key of the issue on the other end of the link"`
	IssueSummary string `json:"issue_summary,omitempty" jsonschema:"the summary of the linked issue"`
	IssueStatus  string `json:"issue_status,omitempty" jsonschema:"the status of the linked issue"`
}

// IssueSummary is a flattened, human-readable view of a Jira issue.
type IssueSummary struct {
	Key               string              `json:"key" jsonschema:"the issue key, e.g. PROJ-123"`
	Summary           string              `json:"summary,omitempty" jsonschema:"the issue summary/title"`
	Status            string              `json:"status,omitempty" jsonschema:"the current workflow status name"`
	IssueType         string              `json:"issue_type,omitempty" jsonschema:"the issue type name, e.g. Bug, Task"`
	Project           string              `json:"project,omitempty" jsonschema:"the project key"`
	Assignee          string              `json:"assignee,omitempty" jsonschema:"the assignee's display name, if assigned"`
	AssigneeAccountID string              `json:"assignee_account_id,omitempty" jsonschema:"the assignee's Jira account id"`
	Reporter          string              `json:"reporter,omitempty" jsonschema:"the reporter's display name"`
	Priority          string              `json:"priority,omitempty" jsonschema:"the priority name, e.g. High"`
	Resolution        string              `json:"resolution,omitempty" jsonschema:"the resolution name, if the issue is resolved"`
	Labels            []string            `json:"labels,omitempty" jsonschema:"the issue's labels"`
	Components        []string            `json:"components,omitempty" jsonschema:"the names of the issue's components"`
	FixVersions       []string            `json:"fix_versions,omitempty" jsonschema:"the names of the issue's fix versions"`
	Parent            string              `json:"parent,omitempty" jsonschema:"the key of the parent issue or epic, if any"`
	Subtasks          []string            `json:"subtasks,omitempty" jsonschema:"the keys of this issue's subtasks"`
	Links             []IssueLinkSummary  `json:"links,omitempty" jsonschema:"issues linked to this one"`
	Attachments       []AttachmentSummary `json:"attachments,omitempty" jsonschema:"the issue's attachments"`
	Description       string              `json:"description,omitempty" jsonschema:"the issue description, rendered as Markdown"`
	DueDate           string              `json:"due_date,omitempty" jsonschema:"the due date, as YYYY-MM-DD"`
	Created           string              `json:"created,omitempty" jsonschema:"creation timestamp"`
	Updated           string              `json:"updated,omitempty" jsonschema:"last update timestamp"`
}

func namedRefNames(refs []jira.NamedRef) []string {
	if len(refs) == 0 {
		return nil
	}
	names := make([]string, len(refs))
	for i, ref := range refs {
		names[i] = ref.Name
	}
	return names
}

func issueLinkSummaries(links []jira.IssueLink) []IssueLinkSummary {
	if len(links) == 0 {
		return nil
	}
	out := make([]IssueLinkSummary, 0, len(links))
	for _, link := range links {
		var ls IssueLinkSummary
		var other *jira.IssueRef
		switch {
		case link.OutwardIssue != nil:
			other = link.OutwardIssue
			if link.Type != nil {
				ls.Type = link.Type.Outward
			}
		case link.InwardIssue != nil:
			other = link.InwardIssue
			if link.Type != nil {
				ls.Type = link.Type.Inward
			}
		default:
			continue
		}
		if ls.Type == "" && link.Type != nil {
			ls.Type = link.Type.Name
		}
		ls.IssueKey = other.Key
		if other.Fields != nil {
			ls.IssueSummary = other.Fields.Summary
			if other.Fields.Status != nil {
				ls.IssueStatus = other.Fields.Status.Name
			}
		}
		out = append(out, ls)
	}
	return out
}

func attachmentSummaries(attachments []jira.Attachment) []AttachmentSummary {
	if len(attachments) == 0 {
		return nil
	}
	out := make([]AttachmentSummary, len(attachments))
	for i, a := range attachments {
		out[i] = AttachmentSummary{
			ID:       a.ID,
			Filename: a.Filename,
			MimeType: a.MimeType,
			Size:     a.Size,
			Created:  a.Created,
		}
	}
	return out
}

func issueToSummary(issue *jira.Issue) IssueSummary {
	s := IssueSummary{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: issue.Fields.DescriptionMarkdown(),
		Labels:      issue.Fields.Labels,
		Components:  namedRefNames(issue.Fields.Components),
		FixVersions: namedRefNames(issue.Fields.FixVersions),
		Links:       issueLinkSummaries(issue.Fields.IssueLinks),
		Attachments: attachmentSummaries(issue.Fields.Attachments),
		DueDate:     issue.Fields.DueDate,
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
		s.AssigneeAccountID = issue.Fields.Assignee.AccountID
	}
	if issue.Fields.Reporter != nil {
		s.Reporter = issue.Fields.Reporter.DisplayName
	}
	if issue.Fields.Priority != nil {
		s.Priority = issue.Fields.Priority.Name
	}
	if issue.Fields.Resolution != nil {
		s.Resolution = issue.Fields.Resolution.Name
	}
	if issue.Fields.Parent != nil {
		s.Parent = issue.Fields.Parent.Key
	}
	for _, sub := range issue.Fields.Subtasks {
		s.Subtasks = append(s.Subtasks, sub.Key)
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
	JQL           string   `json:"jql" jsonschema:"a JQL (Jira Query Language) query string, e.g. 'project = PROJ AND status = \"To Do\"'"`
	NextPageToken string   `json:"next_page_token,omitempty" jsonschema:"token returned by the previous page; omit for the first page"`
	MaxResults    int      `json:"max_results,omitempty" jsonschema:"maximum number of results to return, defaults to 50"`
	Fields        []string `json:"fields,omitempty" jsonschema:"optional list of Jira field names to fetch per issue"`
}

// SearchIssuesOutput is the output for the jira_search_issues tool.
type SearchIssuesOutput struct {
	IsLast        bool           `json:"is_last" jsonschema:"whether this is the final page"`
	NextPageToken string         `json:"next_page_token,omitempty" jsonschema:"token to pass to the next search call"`
	Issues        []IssueSummary `json:"issues" jsonschema:"the matching issues in this page"`
}

func searchIssues(client *jira.Client) mcp.ToolHandlerFor[SearchIssuesInput, SearchIssuesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SearchIssuesInput) (*mcp.CallToolResult, SearchIssuesOutput, error) {
		maxResults := in.MaxResults
		if maxResults <= 0 {
			maxResults = 50
		}
		result, err := client.SearchIssues(ctx, in.JQL, in.NextPageToken, maxResults, in.Fields)
		if err != nil {
			return nil, SearchIssuesOutput{}, fmt.Errorf("search issues: %w", err)
		}
		out := SearchIssuesOutput{
			IsLast:        result.IsLast,
			NextPageToken: result.NextPageToken,
			Issues:        make([]IssueSummary, len(result.Issues)),
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
	Query      string `json:"query,omitempty" jsonschema:"filter projects whose key or name contains this text"`
	TypeKey    string `json:"type_key,omitempty" jsonschema:"filter by project type: software, business or service_desk"`
	OrderBy    string `json:"order_by,omitempty" jsonschema:"sort order, e.g. key, name or lastIssueUpdatedTime"`
	StartAt    int    `json:"start_at,omitempty" jsonschema:"pagination offset, defaults to 0"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"maximum number of results to return, defaults to 50"`
}

// ListProjectsOutput is the output for the jira_list_projects tool.
type ListProjectsOutput struct {
	Total    int              `json:"total" jsonschema:"total number of matching projects"`
	Projects []ProjectSummary `json:"projects" jsonschema:"the projects in this page"`
}

func listProjects(client *jira.Client) mcp.ToolHandlerFor[ListProjectsInput, ListProjectsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListProjectsInput) (*mcp.CallToolResult, ListProjectsOutput, error) {
		result, err := client.ListProjects(ctx, jira.ProjectSearchOptions{
			Query:      in.Query,
			TypeKey:    in.TypeKey,
			OrderBy:    in.OrderBy,
			StartAt:    in.StartAt,
			MaxResults: in.MaxResults,
		})
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

// GetCommentsInput is the input for the jira_get_comments tool.
type GetCommentsInput struct {
	IssueKey   string `json:"issue_key" jsonschema:"the Jira issue key or id, e.g. PROJ-123"`
	StartAt    int    `json:"start_at,omitempty" jsonschema:"pagination offset, defaults to 0"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"maximum number of comments to return, defaults to 50"`
}

// CommentSummary is a flattened view of a Jira comment.
type CommentSummary struct {
	ID      string `json:"id" jsonschema:"the comment id"`
	Author  string `json:"author,omitempty" jsonschema:"the comment author's display name"`
	Body    string `json:"body,omitempty" jsonschema:"the comment text, rendered as Markdown"`
	Created string `json:"created,omitempty" jsonschema:"creation timestamp"`
	Updated string `json:"updated,omitempty" jsonschema:"last update timestamp"`
}

// GetCommentsOutput is the output for the jira_get_comments tool.
type GetCommentsOutput struct {
	Total    int              `json:"total" jsonschema:"total number of comments on the issue"`
	StartAt  int              `json:"start_at" jsonschema:"the offset of this page"`
	Comments []CommentSummary `json:"comments" jsonschema:"the comments in this page, oldest first"`
}

func getComments(client *jira.Client) mcp.ToolHandlerFor[GetCommentsInput, GetCommentsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetCommentsInput) (*mcp.CallToolResult, GetCommentsOutput, error) {
		result, err := client.ListComments(ctx, in.IssueKey, in.StartAt, in.MaxResults)
		if err != nil {
			return nil, GetCommentsOutput{}, fmt.Errorf("get comments for %s: %w", in.IssueKey, err)
		}
		out := GetCommentsOutput{
			Total:    result.Total,
			StartAt:  result.StartAt,
			Comments: make([]CommentSummary, len(result.Comments)),
		}
		for i := range result.Comments {
			comment := &result.Comments[i]
			cs := CommentSummary{
				ID:      comment.ID,
				Body:    comment.BodyMarkdown(),
				Created: comment.Created,
				Updated: comment.Updated,
			}
			if comment.Author != nil {
				cs.Author = comment.Author.DisplayName
			}
			out.Comments[i] = cs
		}
		return nil, out, nil
	}
}

// UserSummary is a flattened view of a Jira user.
type UserSummary struct {
	AccountID   string `json:"account_id" jsonschema:"the Jira account id, used for assignee and reporter fields"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"the user's display name"`
	Email       string `json:"email,omitempty" jsonschema:"the user's email address, if visible"`
}

func userToSummary(u *jira.User) UserSummary {
	return UserSummary{AccountID: u.AccountID, DisplayName: u.DisplayName, Email: u.EmailAddress}
}

// GetMyselfInput is the (empty) input for the jira_get_myself tool.
type GetMyselfInput struct{}

func getMyself(client *jira.Client) mcp.ToolHandlerFor[GetMyselfInput, UserSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ GetMyselfInput) (*mcp.CallToolResult, UserSummary, error) {
		user, err := client.GetMyself(ctx)
		if err != nil {
			return nil, UserSummary{}, fmt.Errorf("get current user: %w", err)
		}
		return nil, userToSummary(user), nil
	}
}

// SearchUsersInput is the input for the jira_search_users tool.
type SearchUsersInput struct {
	Query      string `json:"query" jsonschema:"a display name or email substring to match"`
	StartAt    int    `json:"start_at,omitempty" jsonschema:"pagination offset, defaults to 0"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"maximum number of users to return, defaults to 50"`
}

// SearchUsersOutput is the output for the jira_search_users tool.
type SearchUsersOutput struct {
	Users []UserSummary `json:"users" jsonschema:"the matching users"`
}

func searchUsers(client *jira.Client) mcp.ToolHandlerFor[SearchUsersInput, SearchUsersOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SearchUsersInput) (*mcp.CallToolResult, SearchUsersOutput, error) {
		users, err := client.SearchUsers(ctx, in.Query, in.StartAt, in.MaxResults)
		if err != nil {
			return nil, SearchUsersOutput{}, fmt.Errorf("search users: %w", err)
		}
		out := SearchUsersOutput{Users: make([]UserSummary, len(users))}
		for i := range users {
			out.Users[i] = userToSummary(&users[i])
		}
		return nil, out, nil
	}
}

// ListFieldsInput is the input for the jira_list_fields tool.
type ListFieldsInput struct {
	Query string `json:"query,omitempty" jsonschema:"only return fields whose id or name contains this text"`
}

// FieldSummary is a flattened view of a Jira field definition.
type FieldSummary struct {
	ID     string `json:"id" jsonschema:"the field id, e.g. summary or customfield_10011"`
	Name   string `json:"name,omitempty" jsonschema:"the field's display name"`
	Custom bool   `json:"custom" jsonschema:"whether this is a custom field"`
	Type   string `json:"type,omitempty" jsonschema:"the field's value type, e.g. string, array, option"`
}

// ListFieldsOutput is the output for the jira_list_fields tool.
type ListFieldsOutput struct {
	Fields []FieldSummary `json:"fields" jsonschema:"the matching field definitions"`
}

func listFields(client *jira.Client) mcp.ToolHandlerFor[ListFieldsInput, ListFieldsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListFieldsInput) (*mcp.CallToolResult, ListFieldsOutput, error) {
		fields, err := client.ListFields(ctx, in.Query)
		if err != nil {
			return nil, ListFieldsOutput{}, fmt.Errorf("list fields: %w", err)
		}
		out := ListFieldsOutput{Fields: make([]FieldSummary, len(fields))}
		for i, f := range fields {
			fs := FieldSummary{ID: f.ID, Name: f.Name, Custom: f.Custom}
			if f.Schema != nil {
				fs.Type = f.Schema.Type
			}
			out.Fields[i] = fs
		}
		return nil, out, nil
	}
}

// ListIssueTypesInput is the input for the jira_list_issue_types tool.
type ListIssueTypesInput struct {
	ProjectKey string `json:"project_key" jsonschema:"the Jira project key or id"`
}

// IssueTypeSummary describes an issue type available in a project.
type IssueTypeSummary struct {
	ID          string `json:"id" jsonschema:"the issue type id"`
	Name        string `json:"name,omitempty" jsonschema:"the issue type name, as passed to jira_create_issue"`
	Description string `json:"description,omitempty" jsonschema:"the issue type description"`
	Subtask     bool   `json:"subtask" jsonschema:"whether issues of this type are subtasks and require a parent"`
}

// ListIssueTypesOutput is the output for the jira_list_issue_types tool.
type ListIssueTypesOutput struct {
	IssueTypes []IssueTypeSummary `json:"issue_types" jsonschema:"the issue types that can be created in the project"`
}

func listIssueTypes(client *jira.Client) mcp.ToolHandlerFor[ListIssueTypesInput, ListIssueTypesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListIssueTypesInput) (*mcp.CallToolResult, ListIssueTypesOutput, error) {
		issueTypes, err := client.ListCreateIssueTypes(ctx, in.ProjectKey)
		if err != nil {
			return nil, ListIssueTypesOutput{}, fmt.Errorf("list issue types for %s: %w", in.ProjectKey, err)
		}
		out := ListIssueTypesOutput{IssueTypes: make([]IssueTypeSummary, len(issueTypes))}
		for i, it := range issueTypes {
			out.IssueTypes[i] = IssueTypeSummary{
				ID:          it.ID,
				Name:        it.Name,
				Description: it.Description,
				Subtask:     it.Subtask,
			}
		}
		return nil, out, nil
	}
}

// GetCreateFieldsInput is the input for the jira_get_create_fields tool.
type GetCreateFieldsInput struct {
	ProjectKey string `json:"project_key" jsonschema:"the Jira project key or id"`
	IssueType  string `json:"issue_type" jsonschema:"the issue type name or id, e.g. Bug"`
}

// CreateFieldSummary describes a field on a project's create screen.
type CreateFieldSummary struct {
	ID            string   `json:"id" jsonschema:"the field id, to be used in the fields argument of jira_create_issue"`
	Name          string   `json:"name,omitempty" jsonschema:"the field's display name"`
	Required      bool     `json:"required" jsonschema:"whether the field must be provided when creating an issue"`
	Type          string   `json:"type,omitempty" jsonschema:"the field's value type"`
	AllowedValues []string `json:"allowed_values,omitempty" jsonschema:"the permitted values, when the field is a fixed-option field"`
}

// GetCreateFieldsOutput is the output for the jira_get_create_fields tool.
type GetCreateFieldsOutput struct {
	Fields []CreateFieldSummary `json:"fields" jsonschema:"the fields available on the create screen"`
}

// allowedValueLabels extracts the human-readable label of each allowed
// value, preferring "name", then "value", then "id".
func allowedValueLabels(values []map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	labels := make([]string, 0, len(values))
	for _, v := range values {
		for _, key := range []string{"name", "value", "id"} {
			if label, ok := v[key].(string); ok && label != "" {
				labels = append(labels, label)
				break
			}
		}
	}
	return labels
}

func getCreateFields(client *jira.Client) mcp.ToolHandlerFor[GetCreateFieldsInput, GetCreateFieldsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetCreateFieldsInput) (*mcp.CallToolResult, GetCreateFieldsOutput, error) {
		fields, err := client.ListCreateFields(ctx, in.ProjectKey, in.IssueType)
		if err != nil {
			return nil, GetCreateFieldsOutput{}, fmt.Errorf("get create fields for %s/%s: %w", in.ProjectKey, in.IssueType, err)
		}
		out := GetCreateFieldsOutput{Fields: make([]CreateFieldSummary, len(fields))}
		for i, f := range fields {
			cf := CreateFieldSummary{
				ID:            f.FieldID,
				Name:          f.Name,
				Required:      f.Required,
				AllowedValues: allowedValueLabels(f.AllowedValues),
			}
			if f.Schema != nil {
				cf.Type = f.Schema.Type
			}
			out.Fields[i] = cf
		}
		return nil, out, nil
	}
}

// ListLinkTypesInput is the (empty) input for the jira_list_link_types tool.
type ListLinkTypesInput struct{}

// LinkTypeSummary describes an issue link type.
type LinkTypeSummary struct {
	Name    string `json:"name" jsonschema:"the link type name, as passed to jira_link_issues"`
	Inward  string `json:"inward,omitempty" jsonschema:"the inward description, e.g. 'is blocked by'"`
	Outward string `json:"outward,omitempty" jsonschema:"the outward description, e.g. 'blocks'"`
}

// ListLinkTypesOutput is the output for the jira_list_link_types tool.
type ListLinkTypesOutput struct {
	LinkTypes []LinkTypeSummary `json:"link_types" jsonschema:"the issue link types configured on this Jira site"`
}

func listLinkTypes(client *jira.Client) mcp.ToolHandlerFor[ListLinkTypesInput, ListLinkTypesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ ListLinkTypesInput) (*mcp.CallToolResult, ListLinkTypesOutput, error) {
		linkTypes, err := client.ListIssueLinkTypes(ctx)
		if err != nil {
			return nil, ListLinkTypesOutput{}, fmt.Errorf("list issue link types: %w", err)
		}
		out := ListLinkTypesOutput{LinkTypes: make([]LinkTypeSummary, len(linkTypes))}
		for i, lt := range linkTypes {
			out.LinkTypes[i] = LinkTypeSummary{Name: lt.Name, Inward: lt.Inward, Outward: lt.Outward}
		}
		return nil, out, nil
	}
}

// GetWorklogsInput is the input for the jira_get_worklogs tool.
type GetWorklogsInput struct {
	IssueKey   string `json:"issue_key" jsonschema:"the Jira issue key or id, e.g. PROJ-123"`
	StartAt    int    `json:"start_at,omitempty" jsonschema:"pagination offset, defaults to 0"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"maximum number of entries to return, defaults to 50"`
}

// WorklogSummary is a flattened view of a work log entry.
type WorklogSummary struct {
	ID               string `json:"id" jsonschema:"the worklog id"`
	Author           string `json:"author,omitempty" jsonschema:"the display name of the user who logged the work"`
	TimeSpent        string `json:"time_spent,omitempty" jsonschema:"the logged duration, e.g. '3h 30m'"`
	TimeSpentSeconds int64  `json:"time_spent_seconds,omitempty" jsonschema:"the logged duration in seconds"`
	Started          string `json:"started,omitempty" jsonschema:"when the work started"`
	Comment          string `json:"comment,omitempty" jsonschema:"the worklog comment, rendered as Markdown"`
}

// GetWorklogsOutput is the output for the jira_get_worklogs tool.
type GetWorklogsOutput struct {
	Total    int              `json:"total" jsonschema:"total number of worklog entries on the issue"`
	StartAt  int              `json:"start_at" jsonschema:"the offset of this page"`
	Worklogs []WorklogSummary `json:"worklogs" jsonschema:"the worklog entries in this page"`
}

func getWorklogs(client *jira.Client) mcp.ToolHandlerFor[GetWorklogsInput, GetWorklogsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetWorklogsInput) (*mcp.CallToolResult, GetWorklogsOutput, error) {
		result, err := client.ListWorklogs(ctx, in.IssueKey, in.StartAt, in.MaxResults)
		if err != nil {
			return nil, GetWorklogsOutput{}, fmt.Errorf("get worklogs for %s: %w", in.IssueKey, err)
		}
		out := GetWorklogsOutput{
			Total:    result.Total,
			StartAt:  result.StartAt,
			Worklogs: make([]WorklogSummary, len(result.Worklogs)),
		}
		for i := range result.Worklogs {
			w := &result.Worklogs[i]
			ws := WorklogSummary{
				ID:               w.ID,
				TimeSpent:        w.TimeSpent,
				TimeSpentSeconds: w.TimeSpentSeconds,
				Started:          w.Started,
				Comment:          w.CommentMarkdown(),
			}
			if w.Author != nil {
				ws.Author = w.Author.DisplayName
			}
			out.Worklogs[i] = ws
		}
		return nil, out, nil
	}
}

// DownloadAttachmentInput is the input for the jira_download_attachment tool.
type DownloadAttachmentInput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the Jira attachment id, as returned in an issue's attachments list"`
}

// DownloadAttachmentOutput is the output for the jira_download_attachment tool.
type DownloadAttachmentOutput struct {
	Filename   string `json:"filename" jsonschema:"the attachment's original filename"`
	MimeType   string `json:"mime_type,omitempty" jsonschema:"the attachment's MIME type"`
	Size       int64  `json:"size,omitempty" jsonschema:"the attachment size in bytes"`
	DataBase64 string `json:"data_base64,omitempty" jsonschema:"the attachment content, base64-encoded; omitted when returned as MCP image content"`
}

func downloadAttachment(client *jira.Client) mcp.ToolHandlerFor[DownloadAttachmentInput, DownloadAttachmentOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DownloadAttachmentInput) (*mcp.CallToolResult, DownloadAttachmentOutput, error) {
		attachment, err := client.DownloadAttachment(ctx, in.AttachmentID)
		if err != nil {
			return nil, DownloadAttachmentOutput{}, fmt.Errorf("download attachment %s: %w", in.AttachmentID, err)
		}

		output := DownloadAttachmentOutput{
			Filename:   attachment.Filename,
			MimeType:   attachment.MimeType,
			Size:       attachment.Size,
			DataBase64: base64.StdEncoding.EncodeToString(attachment.Data),
		}

		var result *mcp.CallToolResult
		mediaType, _, parseErr := mime.ParseMediaType(attachment.MimeType)
		if parseErr == nil && strings.HasPrefix(mediaType, "image/") {
			output.DataBase64 = ""
			result = &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.ImageContent{Data: attachment.Data, MIMEType: mediaType},
				},
			}
		}

		return result, output, nil
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
		Name:        "jira_get_comments",
		Description: "List the comments on a Jira issue, with pagination.",
		Annotations: readOnlyHint,
	}, getComments(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_get_myself",
		Description: "Get the Jira user that this server authenticates as, including its account id.",
		Annotations: readOnlyHint,
	}, getMyself(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_search_users",
		Description: "Find Jira users by display name or email, to resolve the account id needed to assign issues.",
		Annotations: readOnlyHint,
	}, searchUsers(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_list_fields",
		Description: "List Jira field definitions, to map field names to the ids used by the raw 'fields' arguments.",
		Annotations: readOnlyHint,
	}, listFields(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_list_issue_types",
		Description: "List the issue types that can be created in a Jira project.",
		Annotations: readOnlyHint,
	}, listIssueTypes(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_get_create_fields",
		Description: "List the fields available when creating an issue of a given type in a project, including which are required and their allowed values.",
		Annotations: readOnlyHint,
	}, getCreateFields(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_list_link_types",
		Description: "List the issue link types configured on this Jira site.",
		Annotations: readOnlyHint,
	}, listLinkTypes(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_get_worklogs",
		Description: "List the work log entries on a Jira issue, with pagination.",
		Annotations: readOnlyHint,
	}, getWorklogs(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_download_attachment",
		Description: "Download a Jira attachment's content by id, base64-encoded.",
		Annotations: readOnlyHint,
	}, downloadAttachment(client))
}
