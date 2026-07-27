package jira

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
)

// DownloadedAttachment holds the content and metadata of a fetched
// attachment, with content base64-encoded for transport over MCP.
type DownloadedAttachment struct {
	Filename   string
	MimeType   string
	Size       int64
	DataBase64 string
}

// DownloadAttachment fetches an attachment's metadata and content by id,
// returning the content base64-encoded. Attachments larger than
// MaxAttachmentBytes are rejected.
func (c *Client) DownloadAttachment(ctx context.Context, id string) (*DownloadedAttachment, error) {
	var meta Attachment
	if err := c.doJSON(ctx, "GET", "rest/api/3/attachment/"+url.PathEscape(id), nil, nil, &meta); err != nil {
		return nil, err
	}

	if meta.Size > MaxAttachmentBytes {
		return nil, fmt.Errorf("%w: attachment %s is %d bytes, limit is %d", ErrTooLarge, id, meta.Size, int64(MaxAttachmentBytes))
	}

	data, contentType, err := c.doRaw(ctx, meta.Content, MaxAttachmentBytes)
	if err != nil {
		return nil, err
	}

	mimeType := meta.MimeType
	if mimeType == "" {
		mimeType = contentType
	}

	return &DownloadedAttachment{
		Filename:   meta.Filename,
		MimeType:   mimeType,
		Size:       meta.Size,
		DataBase64: base64.StdEncoding.EncodeToString(data),
	}, nil
}

// UploadAttachment uploads a file to an issue and returns the resulting
// attachment metadata.
func (c *Client) UploadAttachment(ctx context.Context, issueKeyOrID, filename, mimeType string, data []byte) (*Attachment, error) {
	var created []Attachment
	path := "rest/api/3/issue/" + url.PathEscape(issueKeyOrID) + "/attachments"
	if err := c.doMultipart(ctx, path, filename, mimeType, data, &created); err != nil {
		return nil, err
	}
	if len(created) == 0 {
		return nil, nil
	}
	return &created[0], nil
}
