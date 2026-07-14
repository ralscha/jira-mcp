package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
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

	exe := filepath.Join(t.TempDir(), "jira-mcp.exe")
	//nolint:gosec // The executable and arguments are fixed test inputs; only the output path is generated.
	build := exec.CommandContext(ctx, "go", "build", "-o", exe, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(out))
	}

	//nolint:gosec // exe is built in the test temp dir; addr is a local port allocated by freePort.
	cmd := exec.CommandContext(ctx, exe,
		"--jira-base-url=https://example.atlassian.net",
		"--jira-email=user@example.com",
		"--jira-api-token=tok",
		"--mode=readwrite",
		"--transport=http",
		"--http-addr="+addr,
	)
	cmd.Env = os.Environ()
	var logs lockedBuffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start() error = %v", err)
	}
	exitCh := make(chan error, 1)
	go func() {
		exitCh <- cmd.Wait()
	}()
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			<-exitCh
		}
	}()

	endpoint := "http://" + addr

	var session *mcp.ClientSession
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-test", Version: "0.0.0"}, nil)
	deadline := time.Now().Add(20 * time.Second)
	for {
		select {
		case err := <-exitCh:
			t.Fatalf("jira-mcp exited before accepting HTTP connections: %v\n%s", err, logs.String())
		default:
		}

		session, err = client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("client.Connect() error = %v (giving up after retries)\n%s", err, logs.String())
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

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
