package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestHTTPTransport_ListTools is an end-to-end smoke test: it builds and
// runs the jira-mcp binary serving streamable HTTP, then verifies it
// responds to a real MCP tools/list request.
func TestHTTPTransport_ListTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort() error = %v", err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)

	//nolint:gosec // addr is a local port allocated by freePort, safe for subprocess launch
	cmd := exec.CommandContext(ctx, "go", "run", ".",
		"--jira-base-url=https://example.atlassian.net",
		"--jira-email=user@example.com",
		"--jira-api-token=tok",
		"--mode=readwrite",
		"--transport=http",
		"--http-addr="+addr,
	)
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start() error = %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	endpoint := "http://" + addr

	var session *mcp.ClientSession
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-test", Version: "0.0.0"}, nil)
	deadline := time.Now().Add(20 * time.Second)
	for {
		session, err = client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("client.Connect() error = %v (giving up after retries)", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	foundRead, foundWrite := false, false
	for _, tool := range result.Tools {
		if tool.Name == "jira_get_issue" {
			foundRead = true
		}
		if tool.Name == "jira_create_issue" {
			foundWrite = true
		}
	}
	if !foundRead {
		t.Error("expected jira_get_issue to be registered")
	}
	if !foundWrite {
		t.Error("expected jira_create_issue to be registered in readwrite mode")
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}
