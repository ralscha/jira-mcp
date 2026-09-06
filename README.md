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
| `MCP_AUTH_TOKEN`     | `--auth-token`     | *(empty)*    | Bearer token HTTP clients must present; ignored by the stdio transport |

Run `jira-mcp --version` to print the build version.

## Tools

### Read-only tools (always available)

| Tool                        | Description                                                |
| --------------------------- | ---------------------------------------------------------- |
| `jira_get_issue`            | Get a single Jira issue by key or id                       |
| `jira_search_issues`        | Search Jira issues with JQL, with pagination               |
| `jira_get_comments`         | List an issue's comments, with pagination                  |
| `jira_get_worklogs`         | List an issue's work log entries                           |
| `jira_get_transitions`      | List available workflow transitions for an issue           |
| `jira_list_projects`        | List Jira projects, filterable by name/key and type        |
| `jira_get_project`          | Get a single Jira project by key or id                     |
| `jira_list_issue_types`     | List the issue types creatable in a project                |
| `jira_get_create_fields`    | List create-screen fields for a project and issue type     |
| `jira_list_fields`          | List field definitions, to map names to (custom) field ids |
| `jira_list_link_types`      | List the issue link types configured on the site           |
| `jira_get_myself`           | Get the account this server authenticates as               |
| `jira_search_users`         | Find users by display name or email, to get account ids    |
| `jira_download_attachment`  | Download an attachment (native MCP image or base64 content) |

Attachment ids come from the `attachments` list returned by `jira_get_issue`.

### Write tools (only in `readwrite` mode)

| Tool                     | Description                                    |
| ------------------------ | ---------------------------------------------- |
| `jira_create_issue`      | Create a new Jira issue                        |
| `jira_update_issue`      | Update summary, description, assignee, priority, labels, components, fix versions, due date or raw fields |
| `jira_assign_issue`      | Assign or unassign an issue                    |
| `jira_transition_issue`  | Execute a workflow transition, optionally setting a resolution and comment |
| `jira_add_comment`       | Add a comment to an issue                      |
| `jira_link_issues`       | Link two issues, e.g. "blocks" or "duplicates"  |
| `jira_add_worklog`       | Log work spent on an issue                     |
| `jira_upload_attachment` | Upload a file attachment to an issue           |

The default mode is `readonly`. Set `JIRA_MODE=readwrite` (or
`--mode=readwrite`) explicitly to enable write tools.

### Custom fields

`jira_create_issue`, `jira_update_issue` and `jira_transition_issue` accept a
`fields` object of raw Jira field values keyed by field id, for anything not
covered by a dedicated argument. Use `jira_list_fields` or
`jira_get_create_fields` to discover ids such as `customfield_10011`.

`jira_get_issue` and `jira_search_issues` accept a `fields` list. Requested
custom or otherwise unmodeled values are returned in the output's `fields`
object. Standard fields continue to use the output's dedicated properties.

### Rich text

Issue descriptions and comment bodies use [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/).
The server converts between ADF and Markdown, so tools accept and return
Markdown strings. Headings, paragraphs, bullet and ordered lists, fenced code
blocks, block quotes, horizontal rules and the inline marks `` `code` ``,
`**strong**`, `*emphasis*` and `[links](https://example.com)` are supported.
Plain text is valid Markdown, so unformatted input passes through unchanged.
Other ADF constructs (tables, mentions, media) are rendered as best-effort
text when reading and are not produced when writing. Only `http`, `https` and
`mailto` links are converted; other link schemes stay literal text.

### Limits and rate limiting

Attachment uploads and downloads are capped at 25 MiB. Requests that Jira
rejects with `429 Too Many Requests`, or that fail with a transient gateway
error, are retried up to three times, honouring the `Retry-After` header.

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

Set `MCP_AUTH_TOKEN` (or `--auth-token`) to require clients to send
`Authorization: Bearer <token>`; requests without a matching token get a
`401`. Without it the endpoint has **no authentication**, so you must secure it
at the network or deployment layer (reverse proxy, firewall, or loopback-only
binding) to prevent unauthorized Jira access. This is especially important in
`readwrite` mode, where clients can modify issues. The server logs a warning at
startup whenever HTTP authentication is disabled.

## Authentication

The server authenticates to Jira Cloud using [HTTP Basic auth](https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/)
with your Jira account email as the username and an [API token](https://id.atlassian.com/manage-profile/security/api-tokens)
as the password.

### Required token permissions

Atlassian API tokens do not grant more access than the Atlassian account has.
Use a dedicated account with the smallest Jira project permissions needed for
the tools you expose.

You can use either:

- A classic/unscoped API token with `JIRA_BASE_URL` set to your site URL, e.g.
  `https://your-domain.atlassian.net`.
- A scoped API token. Scoped tokens must call the Atlassian API gateway, e.g.
  `JIRA_BASE_URL=https://api.atlassian.com/ex/jira/{cloudId}/`.

For scoped tokens, grant these Jira scopes:

| Mode | Token scopes | Jira permissions the account still needs |
| ---- | ------------ | ---------------------------------------- |
| `readonly` | `read:jira-work` | Jira product access and `Browse Projects` for the projects/issues to read. Issue security and attachment visibility rules still apply. |
| `readwrite` | `read:jira-work`, `write:jira-work` | The readonly permissions, plus only the project permissions required by the write tools you use: `Create Issues`, `Edit Issues`, `Assign Issues`, `Transition Issues`, `Add Comments`, `Link Issues`, `Work On Issues`, and/or `Create Attachments`. Workflow conditions and field permissions still apply. |

`jira-mcp` does not need Jira administration scopes such as `manage:jira-project`
or `manage:jira-configuration`, and it does not need `Delete Issues` because no
Jira delete tool is exposed.

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

- Go 1.27.1+

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
  (success, error mapping, retry/rate-limit handling, size limits,
  multipart attachment uploads)
- Markdown/ADF conversion round trips
- Mode-gated tool registration (`readonly` excludes write tools)
- HTTP bearer-token authentication
- End-to-end smoke tests that spawn the real binary and drive it via the
  official SDK client over both stdio and HTTP transports

## License

Apache 2.0 — see the [go-sdk LICENSE](https://github.com/modelcontextprotocol/go-sdk/blob/main/LICENSE)
for details.
