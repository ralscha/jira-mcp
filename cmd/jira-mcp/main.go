// Command jira-mcp runs a Model Context Protocol server exposing Jira Cloud
// tools, over either stdio or streamable HTTP, in readonly or readwrite
// mode.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jira-mcp/internal/config"
	"jira-mcp/internal/jira"
	"jira-mcp/internal/mcpserver"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	jiraClient, err := jira.NewClient(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, &http.Client{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return err
	}

	server := mcpserver.NewServer(cfg, jiraClient)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Transport {
	case config.TransportStdio:
		return server.Run(ctx, &mcp.StdioTransport{})
	case config.TransportHTTP:
		return runHTTP(ctx, cfg, server)
	default:
		return errors.New("jira-mcp: unsupported transport: " + string(cfg.Transport))
	}
}

func runHTTP(ctx context.Context, cfg *config.Config, server *mcp.Server) error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		//nolint:gosec // %q escapes control characters, preventing log injection
		log.Printf("jira-mcp: listening on %q (mode=%q)", cfg.HTTPAddr, cfg.Mode)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
