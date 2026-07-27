package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jira-mcp/internal/jira"
)

// nonDestructiveHint is shared by write tools that never delete data.
var nonDestructiveHint = &mcp.ToolAnnotations{DestructiveHint: new(false)}

// CreateIssueInput is the input for the jira_create_issue tool.
type CreateIssueInput struct {
	ProjectKey        string         `json:"project_key" jsonschema:"the key of the project to create the issue in, e.g. PROJ"`
	IssueType         string         `json:"issue_type" jsonschema:"the issue type name, e.g. Task, Bug, Story; use jira_get_create_fields to list valid names"`
	Summary           string         `json:"summary" jsonschema:"the issue summary/title"`
	Description       string         `json:"description,omitempty" jsonschema:"the issue description; Markdown headings, lists, code blocks and inline marks are converted to Jira rich text"`
	ParentKey         string         `json:"parent_key,omitempty" jsonschema:"the key of the parent issue or epic; required when creating a subtask"`
	AssigneeAccountID string         `json:"assignee_account_id,omitempty" jsonschema:"the account id of the assignee, as returned by jira_search_users"`
	Priority          string         `json:"priority,omitempty" jsonschema:"the priority name, e.g. High"`
	Labels            []string       `json:"labels,omitempty" jsonschema:"labels to set on the issue"`
	Components        []string       `json:"components,omitempty" jsonschema:"names of components to set on the issue"`
	DueDate           string         `json:"due_date,omitempty" jsonschema:"the due date, as YYYY-MM-DD"`
	Fields            map[string]any `json:"fields,omitempty" jsonschema:"raw Jira fields to set, keyed by field id, e.g. {\"customfield_10011\": \"value\"}; use jira_list_fields to find ids"`
}

// CreatedIssue describes a newly created issue.
type CreatedIssue struct {
	Key string `json:"key" jsonschema:"the key of the created issue, e.g. PROJ-124"`
}

func createIssue(client *jira.Client) mcp.ToolHandlerFor[CreateIssueInput, CreatedIssue] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateIssueInput) (*mcp.CallToolResult, CreatedIssue, error) {
		issue, err := client.CreateIssue(ctx, jira.CreateIssueInput{
			ProjectKey:        in.ProjectKey,
			IssueType:         in.IssueType,
			Summary:           in.Summary,
			Description:       in.Description,
			ParentKey:         in.ParentKey,
			AssigneeAccountID: in.AssigneeAccountID,
			Priority:          in.Priority,
			Labels:            in.Labels,
			Components:        in.Components,
			DueDate:           in.DueDate,
			Fields:            in.Fields,
		})
		if err != nil {
			return nil, CreatedIssue{}, fmt.Errorf("create issue in project %s: %w", in.ProjectKey, err)
		}
		return nil, CreatedIssue{Key: issue.Key}, nil
	}
}

// UpdateIssueInput is the input for the jira_update_issue tool. All fields
// except issue_key are optional; only the ones provided are changed, and at
// least one must be provided.
type UpdateIssueInput struct {
	IssueKey          string         `json:"issue_key" jsonschema:"the Jira issue key or id to update, e.g. PROJ-123"`
	Summary           *string        `json:"summary,omitempty" jsonschema:"new summary/title for the issue"`
	Description       *string        `json:"description,omitempty" jsonschema:"new description for the issue; Markdown is converted to Jira rich text"`
	AssigneeAccountID *string        `json:"assignee_account_id,omitempty" jsonschema:"account id of the new assignee; pass an empty string to unassign"`
	Priority          *string        `json:"priority,omitempty" jsonschema:"new priority name, e.g. High"`
	Labels            *[]string      `json:"labels,omitempty" jsonschema:"replaces the issue's labels with this list"`
	Components        *[]string      `json:"components,omitempty" jsonschema:"replaces the issue's components with these component names"`
	DueDate           *string        `json:"due_date,omitempty" jsonschema:"new due date as YYYY-MM-DD; pass an empty string to clear it"`
	Fields            map[string]any `json:"fields,omitempty" jsonschema:"raw Jira fields to set, keyed by field id, e.g. {\"customfield_10011\": \"value\"}; use jira_list_fields to find ids"`
}

// UpdateIssueOutput confirms an issue update.
type UpdateIssueOutput struct {
	IssueKey string `json:"issue_key" jsonschema:"the key of the updated issue"`
	Updated  bool   `json:"updated" jsonschema:"whether the update succeeded"`
}

func updateIssue(client *jira.Client) mcp.ToolHandlerFor[UpdateIssueInput, UpdateIssueOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateIssueInput) (*mcp.CallToolResult, UpdateIssueOutput, error) {
		err := client.UpdateIssue(ctx, in.IssueKey, jira.UpdateIssueInput{
			Summary:           in.Summary,
			Description:       in.Description,
			AssigneeAccountID: in.AssigneeAccountID,
			Priority:          in.Priority,
			Labels:            in.Labels,
			Components:        in.Components,
			DueDate:           in.DueDate,
			Fields:            in.Fields,
		})
		if err != nil {
			return nil, UpdateIssueOutput{}, fmt.Errorf("update issue %s: %w", in.IssueKey, err)
		}
		return nil, UpdateIssueOutput{IssueKey: in.IssueKey, Updated: true}, nil
	}
}

// AssignIssueInput is the input for the jira_assign_issue tool.
type AssignIssueInput struct {
	IssueKey  string  `json:"issue_key" jsonschema:"the Jira issue key or id to assign, e.g. PROJ-123"`
	AccountID *string `json:"account_id,omitempty" jsonschema:"the Jira account id of the new assignee, as returned by jira_search_users or jira_get_myself; omit to unassign the issue"`
}

// AssignIssueOutput confirms an assignment change.
type AssignIssueOutput struct {
	IssueKey string `json:"issue_key" jsonschema:"the key of the assigned issue"`
	Assigned bool   `json:"assigned" jsonschema:"whether the assignment succeeded"`
}

func assignIssue(client *jira.Client) mcp.ToolHandlerFor[AssignIssueInput, AssignIssueOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in AssignIssueInput) (*mcp.CallToolResult, AssignIssueOutput, error) {
		if err := client.AssignIssue(ctx, in.IssueKey, in.AccountID); err != nil {
			return nil, AssignIssueOutput{}, fmt.Errorf("assign issue %s: %w", in.IssueKey, err)
		}
		return nil, AssignIssueOutput{IssueKey: in.IssueKey, Assigned: true}, nil
	}
}

// TransitionIssueInput is the input for the jira_transition_issue tool.
type TransitionIssueInput struct {
	IssueKey     string         `json:"issue_key" jsonschema:"the Jira issue key or id to transition, e.g. PROJ-123"`
	TransitionID string         `json:"transition_id" jsonschema:"the transition id to execute, as returned by jira_get_transitions"`
	Resolution   string         `json:"resolution,omitempty" jsonschema:"resolution name to set during the transition, e.g. Done; many workflows require this when closing an issue"`
	Comment      string         `json:"comment,omitempty" jsonschema:"a comment to add as part of the transition; Markdown is supported"`
	Fields       map[string]any `json:"fields,omitempty" jsonschema:"raw Jira fields to set during the transition, keyed by field id"`
}

// TransitionIssueOutput confirms an issue transition.
type TransitionIssueOutput struct {
	IssueKey     string `json:"issue_key" jsonschema:"the key of the transitioned issue"`
	Transitioned bool   `json:"transitioned" jsonschema:"whether the transition succeeded"`
}

func transitionIssue(client *jira.Client) mcp.ToolHandlerFor[TransitionIssueInput, TransitionIssueOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in TransitionIssueInput) (*mcp.CallToolResult, TransitionIssueOutput, error) {
		err := client.DoTransition(ctx, in.IssueKey, jira.TransitionInput{
			TransitionID: in.TransitionID,
			Resolution:   in.Resolution,
			Comment:      in.Comment,
			Fields:       in.Fields,
		})
		if err != nil {
			return nil, TransitionIssueOutput{}, fmt.Errorf("transition issue %s: %w", in.IssueKey, err)
		}
		return nil, TransitionIssueOutput{IssueKey: in.IssueKey, Transitioned: true}, nil
	}
}

// AddCommentInput is the input for the jira_add_comment tool.
type AddCommentInput struct {
	IssueKey string `json:"issue_key" jsonschema:"the Jira issue key or id to comment on, e.g. PROJ-123"`
	Comment  string `json:"comment" jsonschema:"the comment text; Markdown headings, lists, code blocks and inline marks are converted to Jira rich text"`
}

// AddedComment describes a newly created comment.
type AddedComment struct {
	CommentID string `json:"comment_id" jsonschema:"the id of the created comment"`
}

func addComment(client *jira.Client) mcp.ToolHandlerFor[AddCommentInput, AddedComment] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in AddCommentInput) (*mcp.CallToolResult, AddedComment, error) {
		comment, err := client.AddComment(ctx, in.IssueKey, in.Comment)
		if err != nil {
			return nil, AddedComment{}, fmt.Errorf("add comment to %s: %w", in.IssueKey, err)
		}
		return nil, AddedComment{CommentID: comment.ID}, nil
	}
}

// LinkIssuesInput is the input for the jira_link_issues tool.
type LinkIssuesInput struct {
	LinkType     string `json:"link_type" jsonschema:"the link type name, as returned by jira_list_link_types, e.g. Blocks"`
	InwardIssue  string `json:"inward_issue" jsonschema:"the key of the issue on the inward side of the link, e.g. for Blocks this issue 'is blocked by' the outward issue"`
	OutwardIssue string `json:"outward_issue" jsonschema:"the key of the issue on the outward side of the link, e.g. for Blocks this issue 'blocks' the inward issue"`
	Comment      string `json:"comment,omitempty" jsonschema:"an optional comment to add alongside the link; Markdown is supported"`
}

// LinkIssuesOutput confirms a link was created.
type LinkIssuesOutput struct {
	Linked bool `json:"linked" jsonschema:"whether the link was created"`
}

func linkIssues(client *jira.Client) mcp.ToolHandlerFor[LinkIssuesInput, LinkIssuesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in LinkIssuesInput) (*mcp.CallToolResult, LinkIssuesOutput, error) {
		err := client.LinkIssues(ctx, in.LinkType, in.InwardIssue, in.OutwardIssue, in.Comment)
		if err != nil {
			return nil, LinkIssuesOutput{}, fmt.Errorf("link %s and %s: %w", in.InwardIssue, in.OutwardIssue, err)
		}
		return nil, LinkIssuesOutput{Linked: true}, nil
	}
}

// AddWorklogInput is the input for the jira_add_worklog tool.
type AddWorklogInput struct {
	IssueKey  string `json:"issue_key" jsonschema:"the Jira issue key or id to log work against, e.g. PROJ-123"`
	TimeSpent string `json:"time_spent" jsonschema:"the duration in Jira format, e.g. '3h 30m' or '1d'"`
	Comment   string `json:"comment,omitempty" jsonschema:"an optional worklog comment; Markdown is supported"`
	Started   string `json:"started,omitempty" jsonschema:"when the work started, ISO 8601 with offset, e.g. 2024-01-02T10:00:00.000+0000; defaults to now"`
}

// AddedWorklog describes a newly created worklog entry.
type AddedWorklog struct {
	WorklogID string `json:"worklog_id" jsonschema:"the id of the created worklog entry"`
	TimeSpent string `json:"time_spent,omitempty" jsonschema:"the duration Jira recorded"`
}

func addWorklog(client *jira.Client) mcp.ToolHandlerFor[AddWorklogInput, AddedWorklog] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in AddWorklogInput) (*mcp.CallToolResult, AddedWorklog, error) {
		worklog, err := client.AddWorklog(ctx, in.IssueKey, jira.AddWorklogInput{
			TimeSpent: in.TimeSpent,
			Comment:   in.Comment,
			Started:   in.Started,
		})
		if err != nil {
			return nil, AddedWorklog{}, fmt.Errorf("add worklog to %s: %w", in.IssueKey, err)
		}
		return nil, AddedWorklog{WorklogID: worklog.ID, TimeSpent: worklog.TimeSpent}, nil
	}
}

// UploadAttachmentInput is the input for the jira_upload_attachment tool.
type UploadAttachmentInput struct {
	IssueKey   string `json:"issue_key" jsonschema:"the Jira issue key or id to attach the file to, e.g. PROJ-123"`
	Filename   string `json:"filename" jsonschema:"the filename to store the attachment as"`
	MimeType   string `json:"mime_type,omitempty" jsonschema:"the attachment's MIME type, e.g. image/png"`
	DataBase64 string `json:"data_base64" jsonschema:"the file content, base64-encoded"`
}

// UploadedAttachment describes a newly uploaded attachment.
type UploadedAttachment struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the id of the created attachment"`
	Filename     string `json:"filename,omitempty" jsonschema:"the stored filename"`
}

func uploadAttachment(client *jira.Client) mcp.ToolHandlerFor[UploadAttachmentInput, UploadedAttachment] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UploadAttachmentInput) (*mcp.CallToolResult, UploadedAttachment, error) {
		data, err := base64.StdEncoding.DecodeString(in.DataBase64)
		if err != nil {
			return nil, UploadedAttachment{}, fmt.Errorf("decode data_base64: %w", err)
		}
		attachment, err := client.UploadAttachment(ctx, in.IssueKey, in.Filename, in.MimeType, data)
		if err != nil {
			return nil, UploadedAttachment{}, fmt.Errorf("upload attachment to %s: %w", in.IssueKey, err)
		}
		if attachment == nil {
			return nil, UploadedAttachment{}, fmt.Errorf("upload attachment to %s: no attachment returned", in.IssueKey)
		}
		return nil, UploadedAttachment{AttachmentID: attachment.ID, Filename: attachment.Filename}, nil
	}
}

// registerWriteTools registers all write Jira tools on the server.
func registerWriteTools(s *mcp.Server, client *jira.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_create_issue",
		Description: "Create a new Jira issue.",
		Annotations: nonDestructiveHint,
	}, createIssue(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_update_issue",
		Description: "Update fields of an existing Jira issue, such as summary, description, assignee, priority, labels, components or due date.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), IdempotentHint: true},
	}, updateIssue(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_assign_issue",
		Description: "Assign a Jira issue to a user by account id, or unassign it when no account id is given.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), IdempotentHint: true},
	}, assignIssue(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_transition_issue",
		Description: "Execute a workflow transition on a Jira issue (e.g. move it to 'In Progress' or 'Done'), optionally setting a resolution and adding a comment.",
		Annotations: nonDestructiveHint,
	}, transitionIssue(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_add_comment",
		Description: "Add a comment to a Jira issue. The comment body is written in Markdown.",
		Annotations: nonDestructiveHint,
	}, addComment(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_link_issues",
		Description: "Create a link between two Jira issues, e.g. to record that one blocks or duplicates the other.",
		Annotations: nonDestructiveHint,
	}, linkIssues(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_add_worklog",
		Description: "Log work spent on a Jira issue.",
		Annotations: nonDestructiveHint,
	}, addWorklog(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_upload_attachment",
		Description: "Upload a file attachment to a Jira issue. Content must be base64-encoded.",
		Annotations: nonDestructiveHint,
	}, uploadAttachment(client))
}
