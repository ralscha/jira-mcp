package mcpserver

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jira-mcp/internal/config"
	"jira-mcp/internal/jira"
)

func listToolNames(t *testing.T, s *mcp.Server) map[string]bool {
	t.Helper()
	ctx := t.Context()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := s.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer func() { _ = serverSession.Close() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer func() { _ = clientSession.Close() }()

	result, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	names := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	return names
}

func testJiraClient(t *testing.T) *jira.Client {
	t.Helper()
	c, err := jira.NewClient("https://example.atlassian.net", "user@example.com", "tok", nil)
	if err != nil {
		t.Fatalf("jira.NewClient() error = %v", err)
	}
	return c
}

var readToolNames = []string{
	"jira_get_issue",
	"jira_search_issues",
	"jira_list_projects",
	"jira_get_project",
	"jira_get_transitions",
	"jira_download_attachment",
}

var writeToolNames = []string{
	"jira_create_issue",
	"jira_update_issue",
	"jira_transition_issue",
	"jira_add_comment",
	"jira_upload_attachment",
}

func TestNewServer_ReadOnlyMode(t *testing.T) {
	cfg := &config.Config{Mode: config.ModeReadOnly}
	s := NewServer(cfg, testJiraClient(t))
	names := listToolNames(t, s)

	for _, name := range readToolNames {
		if !names[name] {
			t.Errorf("expected read tool %q to be registered", name)
		}
	}
	for _, name := range writeToolNames {
		if names[name] {
			t.Errorf("write tool %q should not be registered in readonly mode", name)
		}
	}
}

func TestNewServer_ReadWriteMode(t *testing.T) {
	cfg := &config.Config{Mode: config.ModeReadWrite}
	s := NewServer(cfg, testJiraClient(t))
	names := listToolNames(t, s)

	for _, name := range append(append([]string{}, readToolNames...), writeToolNames...) {
		if !names[name] {
			t.Errorf("expected tool %q to be registered in readwrite mode", name)
		}
	}
}
