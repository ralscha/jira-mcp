// Package config loads and validates jira-mcp server configuration from
// environment variables and command-line flags.
package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Mode controls whether write tools are registered on the MCP server.
type Mode string

const (
	ModeReadOnly  Mode = "readonly"
	ModeReadWrite Mode = "readwrite"
)

// Transport selects how the MCP server communicates with clients.
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

// Config holds all settings needed to run the jira-mcp server.
type Config struct {
	JiraBaseURL  string
	JiraEmail    string
	JiraAPIToken string

	Mode      Mode
	Transport Transport
	HTTPAddr  string
}

// Load builds a Config from environment variables, then applies overrides
// from the given command-line arguments (excluding the program name).
//
// Environment variables:
//   - JIRA_BASE_URL
//   - JIRA_EMAIL
//   - JIRA_API_TOKEN
//   - JIRA_MODE (readonly|readwrite)
//   - MCP_TRANSPORT (stdio|http)
//   - MCP_HTTP_ADDR
func Load(args []string) (*Config, error) {
	cfg := &Config{
		JiraBaseURL:  os.Getenv("JIRA_BASE_URL"),
		JiraEmail:    os.Getenv("JIRA_EMAIL"),
		JiraAPIToken: os.Getenv("JIRA_API_TOKEN"),
		Mode:         ModeReadOnly,
		Transport:    TransportStdio,
		HTTPAddr:     ":8080",
	}

	if v := os.Getenv("JIRA_MODE"); v != "" {
		cfg.Mode = Mode(v)
	}
	if v := os.Getenv("MCP_TRANSPORT"); v != "" {
		cfg.Transport = Transport(v)
	}
	if v := os.Getenv("MCP_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}

	fs := flag.NewFlagSet("jira-mcp", flag.ContinueOnError)
	baseURL := fs.String("jira-base-url", cfg.JiraBaseURL, "Jira Cloud base URL, e.g. https://your-domain.atlassian.net")
	email := fs.String("jira-email", cfg.JiraEmail, "Jira account email used for API token authentication")
	token := fs.String("jira-api-token", cfg.JiraAPIToken, "Jira API token")
	mode := fs.String("mode", string(cfg.Mode), "Server mode: readonly or readwrite")
	transport := fs.String("transport", string(cfg.Transport), "Transport: stdio or http")
	httpAddr := fs.String("http-addr", cfg.HTTPAddr, "Address to listen on when --transport=http")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.JiraBaseURL = *baseURL
	cfg.JiraEmail = *email
	cfg.JiraAPIToken = *token
	cfg.Mode = Mode(*mode)
	cfg.Transport = Transport(*transport)
	cfg.HTTPAddr = *httpAddr

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []string

	if c.JiraBaseURL == "" {
		errs = append(errs, "JIRA_BASE_URL (or --jira-base-url) is required")
	} else if u, err := url.Parse(c.JiraBaseURL); err != nil {
		errs = append(errs, fmt.Sprintf("JIRA_BASE_URL is not a valid URL: %v", err))
	} else if u.Scheme != "https" {
		errs = append(errs, "JIRA_BASE_URL must use https")
	}

	if c.JiraEmail == "" {
		errs = append(errs, "JIRA_EMAIL (or --jira-email) is required")
	}
	if c.JiraAPIToken == "" {
		errs = append(errs, "JIRA_API_TOKEN (or --jira-api-token) is required")
	}

	switch c.Mode {
	case ModeReadOnly, ModeReadWrite:
	default:
		errs = append(errs, fmt.Sprintf("invalid mode %q: must be %q or %q", c.Mode, ModeReadOnly, ModeReadWrite))
	}

	switch c.Transport {
	case TransportStdio, TransportHTTP:
	default:
		errs = append(errs, fmt.Sprintf("invalid transport %q: must be %q or %q", c.Transport, TransportStdio, TransportHTTP))
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// IsReadWrite reports whether write tools should be registered.
func (c *Config) IsReadWrite() bool {
	return c.Mode == ModeReadWrite
}
