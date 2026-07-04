package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestStdioTransport_ListTools is an end-to-end smoke test: it builds and
// runs the jira-mcp binary over stdio and verifies it responds to a real
// MCP tools/list request with the expected read-only tool set (no Jira
// credentials are actually exercised).
func TestStdioTransport_ListTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", ".",
		"--jira-base-url=https://example.atlassian.net",
		"--jira-email=user@example.com",
		"--jira-api-token=tok",
		"--mode=readonly",
		"--transport=stdio",
	)
	cmd.Env = os.Environ()

	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-test", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(result.Tools) == 0 {
		t.Fatal("ListTools() returned no tools")
	}

	found := false
	for _, tool := range result.Tools {
		if tool.Name == "jira_get_issue" {
			found = true
		}
		if tool.Name == "jira_create_issue" {
			t.Errorf("write tool %q should not be registered in readonly mode", tool.Name)
		}
	}
	if !found {
		t.Error("expected jira_get_issue to be registered")
	}
}
