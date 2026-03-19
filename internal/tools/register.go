// Package tools registers Instagram MCP tools with the MCP server.
// Each tool is a thin adapter: validate input → call instagram.Client → format output.
//
// Tool files are split by domain:
//
//	upload.go   — upload_reel, upload_image_post, upload_carousel
//	lifecycle.go — check_container_status, publish_media
//	query.go    — get_media_insights, get_publishing_rate_limit, list_recent_media
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/hosting"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// Register adds all Instagram tools to the MCP server.
// host may be nil — if so, local-file hosting tools are not registered.
func Register(server *mcp.Server, client instagram.Client, host hosting.VideoHost) {
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
	registerListMedia(server, client)

	// Video hosting tools (only when a host backend is configured)
	if host != nil {
		registerHostVideo(server, host)
		registerDeleteHostedAsset(server, host)
	}
}
