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
	ProjectKey  string `json:"project_key" jsonschema:"the key of the project to create the issue in, e.g. PROJ"`
	IssueType   string `json:"issue_type" jsonschema:"the issue type name, e.g. Task, Bug, Story"`
	Summary     string `json:"summary" jsonschema:"the issue summary/title"`
	Description string `json:"description,omitempty" jsonschema:"the issue description as plain text"`
}

// CreatedIssue describes a newly created issue.
type CreatedIssue struct {
	Key string `json:"key" jsonschema:"the key of the created issue, e.g. PROJ-124"`
}

func createIssue(client *jira.Client) mcp.ToolHandlerFor[CreateIssueInput, CreatedIssue] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateIssueInput) (*mcp.CallToolResult, CreatedIssue, error) {
		issue, err := client.CreateIssue(ctx, jira.CreateIssueInput{
			ProjectKey:  in.ProjectKey,
			IssueType:   in.IssueType,
			Summary:     in.Summary,
			Description: in.Description,
		})
		if err != nil {
			return nil, CreatedIssue{}, fmt.Errorf("create issue in project %s: %w", in.ProjectKey, err)
		}
		return nil, CreatedIssue{Key: issue.Key}, nil
	}
}

// UpdateIssueInput is the input for the jira_update_issue tool. Summary and
// Description are optional; only non-empty fields are updated. At least one
// must be provided.
type UpdateIssueInput struct {
	IssueKey    string  `json:"issue_key" jsonschema:"the Jira issue key or id to update, e.g. PROJ-123"`
	Summary     *string `json:"summary,omitempty" jsonschema:"new summary/title for the issue"`
	Description *string `json:"description,omitempty" jsonschema:"new description for the issue, as plain text"`
}

// UpdateIssueOutput confirms an issue update.
type UpdateIssueOutput struct {
	IssueKey string `json:"issue_key" jsonschema:"the key of the updated issue"`
	Updated  bool   `json:"updated" jsonschema:"whether the update succeeded"`
}

func updateIssue(client *jira.Client) mcp.ToolHandlerFor[UpdateIssueInput, UpdateIssueOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateIssueInput) (*mcp.CallToolResult, UpdateIssueOutput, error) {
		err := client.UpdateIssue(ctx, in.IssueKey, jira.UpdateIssueInput{
			Summary:     in.Summary,
			Description: in.Description,
		})
		if err != nil {
			return nil, UpdateIssueOutput{}, fmt.Errorf("update issue %s: %w", in.IssueKey, err)
		}
		return nil, UpdateIssueOutput{IssueKey: in.IssueKey, Updated: true}, nil
	}
}

// TransitionIssueInput is the input for the jira_transition_issue tool.
type TransitionIssueInput struct {
	IssueKey     string `json:"issue_key" jsonschema:"the Jira issue key or id to transition, e.g. PROJ-123"`
	TransitionID string `json:"transition_id" jsonschema:"the transition id to execute, as returned by jira_get_transitions"`
}

// TransitionIssueOutput confirms an issue transition.
type TransitionIssueOutput struct {
	IssueKey     string `json:"issue_key" jsonschema:"the key of the transitioned issue"`
	Transitioned bool   `json:"transitioned" jsonschema:"whether the transition succeeded"`
}

func transitionIssue(client *jira.Client) mcp.ToolHandlerFor[TransitionIssueInput, TransitionIssueOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in TransitionIssueInput) (*mcp.CallToolResult, TransitionIssueOutput, error) {
		err := client.DoTransition(ctx, in.IssueKey, in.TransitionID)
		if err != nil {
			return nil, TransitionIssueOutput{}, fmt.Errorf("transition issue %s: %w", in.IssueKey, err)
		}
		return nil, TransitionIssueOutput{IssueKey: in.IssueKey, Transitioned: true}, nil
	}
}

// AddCommentInput is the input for the jira_add_comment tool.
type AddCommentInput struct {
	IssueKey string `json:"issue_key" jsonschema:"the Jira issue key or id to comment on, e.g. PROJ-123"`
	Comment  string `json:"comment" jsonschema:"the comment text, as plain text"`
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
		Description: "Update the summary and/or description of an existing Jira issue.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), IdempotentHint: true},
	}, updateIssue(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_transition_issue",
		Description: "Execute a workflow transition on a Jira issue (e.g. move it to 'In Progress' or 'Done').",
		Annotations: nonDestructiveHint,
	}, transitionIssue(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_add_comment",
		Description: "Add a plain-text comment to a Jira issue.",
		Annotations: nonDestructiveHint,
	}, addComment(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jira_upload_attachment",
		Description: "Upload a file attachment to a Jira issue. Content must be base64-encoded.",
		Annotations: nonDestructiveHint,
	}, uploadAttachment(client))
}
