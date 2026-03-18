// Command instagram-mcp runs an MCP server that exposes Instagram Graph API
// operations (Reel uploads, image posts, carousels, insights) as tools.
//
// It communicates over stdio and is designed to be launched by an MCP host
// such as Claude Desktop or Claude Code.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/config"
	"github.com/vikrant/instagram-mcp/internal/graphapi"
	"github.com/vikrant/instagram-mcp/internal/instagram"
	"github.com/vikrant/instagram-mcp/internal/tools"
)

const (
	serverName    = "instagram-mcp"
	serverVersion = "1.0.0"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ── Configuration ────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// ── Graph API transport ──────────────────────────────────────────────
	api := graphapi.NewClient(cfg.BaseURL(), cfg.AccessToken)

	// ── Instagram domain client ──────────────────────────────────────────
	igClient := instagram.NewClient(api, cfg.AccountID, cfg.PollInterval, cfg.PollMaxAttempts)

	// ── MCP server ───────────────────────────────────────────────────────
	server := mcp.NewServer(
		&mcp.Implementation{Name: serverName, Version: serverVersion},
		nil,
	)

	tools.Register(server, igClient)

	// ── Graceful shutdown ────────────────────────────────────────────────
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Run ──────────────────────────────────────────────────────────────
	log.SetOutput(os.Stderr) // keep stdout clean for MCP protocol
	log.Printf("%s %s starting on stdio", serverName, serverVersion)

	transport := mcp.NewStdioTransport()
	return server.Run(ctx, transport)
}
