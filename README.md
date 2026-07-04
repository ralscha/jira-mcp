# jira-mcp

A [Model Context Protocol](https://modelcontextprotocol.io/) server that exposes
[Jira Cloud](https://www.atlassian.com/software/jira) tools to MCP clients (such
as Claude, VS Code, or any MCP-compatible host). Built with the official
[`github.com/modelcontextprotocol/go-sdk`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp).

Supports **stdio** and **streamable HTTP** transports, and a **readonly** /
**readwrite** mode switch so you can control whether write operations are
exposed.

## Installation

Download the latest release for your platform from the [Releases](https://github.com/ralscha/jira-mcp/releases) page.

## Quick start

```bash
# Set required environment variables
export JIRA_BASE_URL=https://your-domain.atlassian.net
export JIRA_EMAIL=you@example.com
export JIRA_API_TOKEN=your-api-token

# Run in readonly mode over stdio (safe for exploration)
go run ./cmd/jira-mcp --mode=readonly --transport=stdio
```

Generate an API token at [Atlassian account settings](https://id.atlassian.com/manage-profile/security/api-tokens).

## Configuration

All settings can be provided via **environment variables** or **CLI flags**.
Flags take precedence over environment variables.

| Environment variable | CLI flag           | Default      | Description                                          |
| -------------------- | ------------------ | ------------ | ---------------------------------------------------- |
| `JIRA_BASE_URL`      | `--jira-base-url`  | *(required)* | Jira Cloud base URL, e.g. `https://your-domain.atlassian.net` |
| `JIRA_EMAIL`         | `--jira-email`     | *(required)* | Jira account email (used for Basic auth)             |
| `JIRA_API_TOKEN`     | `--jira-api-token` | *(required)* | Jira API token                                       |
| `JIRA_MODE`          | `--mode`           | `readonly`   | `readonly` or `readwrite`                            |
| `MCP_TRANSPORT`      | `--transport`      | `stdio`      | `stdio` or `http`                                    |
| `MCP_HTTP_ADDR`      | `--http-addr`      | `:8080`      | Listen address when `--transport=http`               |

## Tools

### Read-only tools (always available)

| Tool                        | Description                                                |
| --------------------------- | ---------------------------------------------------------- |
| `jira_get_issue`            | Get a single Jira issue by key or id                       |
| `jira_search_issues`        | Search Jira issues with JQL, with pagination               |
| `jira_list_projects`        | List Jira projects visible to the authenticated user       |
| `jira_get_project`          | Get a single Jira project by key or id                     |
| `jira_get_transitions`      | List available workflow transitions for an issue           |
| `jira_download_attachment`  | Download a Jira attachment's content (base64-encoded)      |

### Write tools (only in `readwrite` mode)

| Tool                     | Description                                    |
| ------------------------ | ---------------------------------------------- |
| `jira_create_issue`      | Create a new Jira issue                        |
| `jira_update_issue`      | Update summary and/or description of an issue  |
| `jira_transition_issue`  | Execute a workflow transition on an issue      |
| `jira_add_comment`       | Add a plain-text comment to an issue           |
| `jira_upload_attachment` | Upload a file attachment to an issue           |

The default mode is `readonly`. Set `JIRA_MODE=readwrite` (or
`--mode=readwrite`) explicitly to enable write tools.

Issue descriptions and comment bodies use [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/).
The server automatically converts plain text to/from a minimal ADF paragraph
document, so you can work with plain strings without handling ADF directly.
Rich formatting (tables, mentions, etc.) is not preserved through these
conversions.

## Transports

### stdio

The default transport. The server communicates over stdin/stdout using
newline-delimited JSON (the standard MCP transport for subprocess-based tools).

```
jira-mcp --transport=stdio
```

### HTTP (streamable)

The server exposes a [streamable HTTP](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
endpoint on the configured address.

```
jira-mcp --transport=http --http-addr=:8080
```

The HTTP transport has **no built-in authentication**. When running in
`readwrite` mode, secure it at the network or deployment layer (reverse proxy,
firewall, loopback-only binding) to prevent unauthorized issue modifications.

## Authentication

The server authenticates to Jira Cloud using [HTTP Basic auth](https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/)
with your Jira account email as the username and an [API token](https://id.atlassian.com/manage-profile/security/api-tokens)
as the password.

## MCP client configuration

### Claude Desktop (stdio)

```json
{
  "mcpServers": {
    "jira": {
      "command": "jira-mcp",
      "args": [],
      "env": {
        "JIRA_BASE_URL": "https://your-domain.atlassian.net",
        "JIRA_EMAIL": "you@example.com",
        "JIRA_API_TOKEN": "your-api-token",
        "JIRA_MODE": "readonly"
      }
    }
  }
}
```

### VS Code / GitHub Copilot (stdio)

Add to `.vscode/mcp.json` (or your user-level `mcp.json`):

```json
{
  "servers": {
    "jira": {
      "command": "jira-mcp",
      "args": [],
      "env": {
        "JIRA_BASE_URL": "https://your-domain.atlassian.net",
        "JIRA_EMAIL": "you@example.com",
        "JIRA_API_TOKEN": "your-api-token",
        "JIRA_MODE": "readonly"
      }
    }
  }
}
```

## Development

### Requirements

- Go 1.26+

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

Tests cover:

- Config loading, validation, and flag/env precedence
- Jira REST API client against `httptest.Server` mocks for every endpoint
  (success, error mapping, multipart attachment uploads)
- Mode-gated tool registration (`readonly` excludes write tools)
- End-to-end smoke tests that spawn the real binary and drive it via the
  official SDK client over both stdio and HTTP transports

## License

Apache 2.0 — see the [go-sdk LICENSE](https://github.com/modelcontextprotocol/go-sdk/blob/main/LICENSE)
for details.
