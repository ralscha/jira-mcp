package config

import (
	"testing"
)

func TestLoad_EnvDefaults(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "user@example.com")
	t.Setenv("JIRA_API_TOKEN", "tok123")
	t.Setenv("JIRA_MODE", "")
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_HTTP_ADDR", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JiraBaseURL != "https://example.atlassian.net" {
		t.Errorf("JiraBaseURL = %q", cfg.JiraBaseURL)
	}
	if cfg.Mode != ModeReadOnly {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeReadOnly)
	}
	if cfg.Transport != TransportStdio {
		t.Errorf("Transport = %q, want %q", cfg.Transport, TransportStdio)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.IsReadWrite() {
		t.Errorf("IsReadWrite() = true, want false")
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "https://env.atlassian.net")
	t.Setenv("JIRA_EMAIL", "env@example.com")
	t.Setenv("JIRA_API_TOKEN", "envtok")
	t.Setenv("JIRA_MODE", "readonly")
	t.Setenv("MCP_TRANSPORT", "stdio")

	cfg, err := Load([]string{
		"--jira-base-url=https://flag.atlassian.net",
		"--mode=readwrite",
		"--transport=http",
		"--http-addr=:9090",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JiraBaseURL != "https://flag.atlassian.net" {
		t.Errorf("JiraBaseURL = %q, want flag value", cfg.JiraBaseURL)
	}
	if cfg.Mode != ModeReadWrite {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeReadWrite)
	}
	if cfg.Transport != TransportHTTP {
		t.Errorf("Transport = %q, want %q", cfg.Transport, TransportHTTP)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if !cfg.IsReadWrite() {
		t.Errorf("IsReadWrite() = false, want true")
	}
	// env-sourced fields not overridden by flags should persist.
	if cfg.JiraEmail != "env@example.com" {
		t.Errorf("JiraEmail = %q, want env value", cfg.JiraEmail)
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "")
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_MODE", "")
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_HTTP_ADDR", "")

	_, err := Load(nil)
	if err == nil {
		t.Fatal("Load() error = nil, want error for missing required fields")
	}
}

func TestLoad_InvalidScheme(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "http://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "user@example.com")
	t.Setenv("JIRA_API_TOKEN", "tok123")
	t.Setenv("JIRA_MODE", "")
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_HTTP_ADDR", "")

	_, err := Load(nil)
	if err == nil {
		t.Fatal("Load() error = nil, want error for non-https base URL")
	}
}

func TestLoad_InvalidMode(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "user@example.com")
	t.Setenv("JIRA_API_TOKEN", "tok123")
	t.Setenv("JIRA_MODE", "bogus")
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_HTTP_ADDR", "")

	_, err := Load(nil)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid mode")
	}
}
