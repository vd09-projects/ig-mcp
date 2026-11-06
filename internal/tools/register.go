// Package tools registers Instagram MCP tools with the MCP server.
// Each tool is a thin adapter: validate input → call instagram.Client → format output.
//
// Tool files are split by domain:
//
//	upload.go   — upload_reel, upload_image_post, upload_carousel
//	lifecycle.go — check_container_status, publish_media
//	query.go    — get_media_insights, get_publishing_rate_limit, list_recent_media
//	analytics.go — get_reels_analytics
package tools

import (
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/hosting"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// Options holds per-tool defaults and caps.
type Options struct {
	// MaxMediaLimit caps how many media items a single tool call will page through.
	MaxMediaLimit int
	// ListMediaDefault is list_recent_media's limit when the caller omits one.
	ListMediaDefault int
	// ReelsDefault is get_reels_analytics's limit when the caller omits one.
	ReelsDefault int
	// InsightsConcurrency bounds parallel insight requests in get_reels_analytics.
	InsightsConcurrency int
	// CleanupOlderThan is cleanup_hosted_assets's age threshold when the caller omits one.
	CleanupOlderThan time.Duration
}

// Register adds all Instagram tools to the MCP server.
// host may be nil — if so, local-file hosting tools are not registered.
func Register(server *mcp.Server, client instagram.Client, host hosting.VideoHost, opts Options) {
	// Upload tools
	registerUploadReel(server, client, host)
	registerUploadImage(server, client)
	registerUploadCarousel(server, client)

	// Container lifecycle tools
	registerCheckContainerStatus(server, client)
	registerPublishMedia(server, client)

	// Query tools
	registerGetRateLimit(server, client)
	registerGetInsights(server, client)
	registerListMedia(server, client, opts)
	registerGetReelsAnalytics(server, client, opts)

	// Video hosting tools (only when a host backend is configured)
	if host != nil {
		registerHostVideo(server, host)
		registerDeleteHostedAsset(server, host)
		registerCleanupHostedAssets(server, host, opts)
	}
}
