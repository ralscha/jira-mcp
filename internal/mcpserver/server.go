package mcpserver

import (
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jira-mcp/internal/config"
	"jira-mcp/internal/jira"
)

// Version is the reported version of this MCP server implementation. It can
// be set at build time with
// -ldflags "-X jira-mcp/internal/mcpserver.Version=v1.2.3"; otherwise the
// version recorded by the Go toolchain is used.
var Version string

// ServerVersion returns the version this server reports to MCP clients.
func ServerVersion() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// NewServer builds an MCP server exposing Jira tools backed by client. Read
// tools are always registered; write tools are only registered when
// cfg.IsReadWrite() is true.
func NewServer(cfg *config.Config, client *jira.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "jira-mcp",
		Version: ServerVersion(),
	}, nil)

	registerReadTools(s, client)
	if cfg.IsReadWrite() {
		registerWriteTools(s, client)
	}

	return s
}
