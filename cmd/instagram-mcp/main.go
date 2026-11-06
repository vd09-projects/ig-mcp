// Command instagram-mcp runs an MCP server that exposes Instagram Graph API
// operations (Reel uploads, image posts, carousels, insights) as tools.
//
// It communicates over stdio and is designed to be launched by an MCP host
// such as Claude Desktop or Claude Code.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/config"
	"github.com/vikrant/instagram-mcp/internal/graphapi"
	"github.com/vikrant/instagram-mcp/internal/hosting"
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
	// ── Flags ─────────────────────────────────────────────────────────────
	envFile := flag.String("env-file", "", "path to .env file (overrides ENV_FILE env var and local .env)")
	configFile := flag.String("config", "", "path to YAML config file (overrides CONFIG_FILE env var)")
	flag.Parse()

	// ── Configuration ────────────────────────────────────────────────────
	cfg, err := config.Load(*envFile, *configFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// ── Graph API transport ──────────────────────────────────────────────
	api := graphapi.NewClient(cfg.BaseURL(), cfg.AccessToken,
		graphapi.WithHTTPClient(&http.Client{Timeout: cfg.GraphAPI.Timeout}))

	// ── Instagram domain client ──────────────────────────────────────────
	igClient := instagram.NewClient(api, cfg.AccountID, cfg.Polling.Interval, cfg.Polling.MaxAttempts)

	// ── Video hosting (optional) ─────────────────────────────────────────
	var videoHost hosting.VideoHost
	if cfg.GitHubHostingEnabled() {
		gh, err := hosting.NewGitHubHost(cfg.GitHubToken, cfg.GitHubRepo, cfg.HostingOptions())
		if err != nil {
			return fmt.Errorf("creating GitHub host: %w", err)
		}
		videoHost = gh
		log.Printf("github hosting enabled (repo: %s)", cfg.GitHubRepo)
	}

	// ── MCP server ───────────────────────────────────────────────────────
	server := mcp.NewServer(
		&mcp.Implementation{Name: serverName, Version: serverVersion},
		nil,
	)

	tools.Register(server, igClient, videoHost, cfg.ToolsOptions())

	// ── Graceful shutdown ────────────────────────────────────────────────
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Run ──────────────────────────────────────────────────────────────
	log.SetOutput(os.Stderr) // keep stdout clean for MCP protocol
	log.Printf("%s %s starting on stdio", serverName, serverVersion)

	transport := mcp.NewStdioTransport()
	return server.Run(ctx, transport)
}
