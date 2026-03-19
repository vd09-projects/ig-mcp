// Command ig-test is a CLI for testing the Instagram MCP pipeline
// without going through the MCP protocol layer.
//
// Usage:
//
//	ig-test host   <file>                   Upload a local file to GitHub Releases
//	ig-test delete <asset_id>               Delete a hosted asset
//	ig-test reel   <file> [caption]         Full pipeline: host → create → wait → publish
//	ig-test reel-url <url> [caption]        Reel from URL: create → wait → publish
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/vikrant/instagram-mcp/internal/config"
	"github.com/vikrant/instagram-mcp/internal/graphapi"
	"github.com/vikrant/instagram-mcp/internal/hosting"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

func main() {
	log.SetFlags(log.Ltime)

	if len(os.Args) < 2 {
		usage()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	switch os.Args[1] {
	case "host":
		requireArgs(3, "ig-test host <file>")
		host := mustGitHubHost(cfg)
		url, assetID, err := host.Upload(ctx, os.Args[2])
		if err != nil {
			log.Fatalf("upload failed: %v", err)
		}
		fmt.Printf("URL:      %s\nAsset ID: %d\n", url, assetID)

	case "delete":
		requireArgs(3, "ig-test delete <asset_id>")
		host := mustGitHubHost(cfg)
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			log.Fatalf("invalid asset_id: %v", err)
		}
		if err := host.Delete(ctx, id); err != nil {
			log.Fatalf("delete failed: %v", err)
		}
		fmt.Println("Deleted.")

	case "reel":
		requireArgs(3, "ig-test reel <file> [caption]")
		host := mustGitHubHost(cfg)
		caption := argOrEmpty(3)

		log.Println("hosting video...")
		url, assetID, err := host.Upload(ctx, os.Args[2])
		if err != nil {
			log.Fatalf("upload failed: %v", err)
		}
		log.Printf("hosted: %s (asset %d)", url, assetID)

		publishReel(ctx, cfg, url, caption)

		log.Printf("cleaning up asset %d...", assetID)
		if err := host.Delete(ctx, assetID); err != nil {
			log.Printf("warning: cleanup failed: %v", err)
		} else {
			log.Println("asset deleted")
		}

	case "reel-url":
		requireArgs(3, "ig-test reel-url <url> [caption]")
		caption := argOrEmpty(3)
		publishReel(ctx, cfg, os.Args[2], caption)

	default:
		usage()
	}
}

func publishReel(ctx context.Context, cfg *config.Config, videoURL, caption string) {
	api := graphapi.NewClient(cfg.BaseURL(), cfg.AccessToken)
	ig := instagram.NewClient(api, cfg.AccountID, cfg.PollInterval, cfg.PollMaxAttempts)

	log.Println("creating reel container...")
	containerID, err := ig.CreateReelContainer(ctx, instagram.ReelParams{
		VideoURL: videoURL,
		Caption:  caption,
	})
	if err != nil {
		log.Fatalf("create container: %v", err)
	}
	log.Printf("container: %s", containerID)

	log.Println("waiting for processing...")
	status, err := ig.WaitForContainer(ctx, containerID)
	if err != nil {
		log.Fatalf("wait: %v", err)
	}
	log.Printf("status: %s", status.StatusCode)

	log.Println("publishing...")
	pub, err := ig.Publish(ctx, containerID)
	if err != nil {
		log.Fatalf("publish: %v", err)
	}
	fmt.Printf("Published! Media ID: %s\n", pub.ID)
}

func mustGitHubHost(cfg *config.Config) *hosting.GitHubHost {
	if !cfg.GitHubHostingEnabled() {
		log.Fatal("GITHUB_TOKEN and GITHUB_REPO must be set")
	}
	host, err := hosting.NewGitHubHost(cfg.GitHubToken, cfg.GitHubRepo)
	if err != nil {
		log.Fatalf("github host: %v", err)
	}
	return host
}

func requireArgs(n int, hint string) {
	if len(os.Args) < n {
		fmt.Fprintf(os.Stderr, "usage: %s\n", hint)
		os.Exit(1)
	}
}

func argOrEmpty(i int) string {
	if i < len(os.Args) {
		return os.Args[i]
	}
	return ""
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  ig-test host   <file>              Upload to GitHub Releases
  ig-test delete <asset_id>          Delete a hosted asset
  ig-test reel   <file> [caption]    Full pipeline: host → IG publish → cleanup
  ig-test reel-url <url> [caption]   Publish reel from public URL`)
	os.Exit(1)
}
