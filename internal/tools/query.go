package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// ---------------------------------------------------------------------------
// get_publishing_rate_limit
// ---------------------------------------------------------------------------

type emptyInput struct{}

type rateLimitOutput struct {
	QuotaTotal    int `json:"quota_total"`
	QuotaDuration int `json:"quota_duration"`
	QuotaUsage    int `json:"quota_usage"`
}

func registerGetRateLimit(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_publishing_rate_limit",
		Description: "Check remaining posts in the 24-hour API publishing window (limit: 50 posts).",
	}, func(ctx context.Context, _ *mcp.ServerSession, _ *mcp.CallToolParamsFor[emptyInput]) (*mcp.CallToolResultFor[rateLimitOutput], error) {
		info, err := client.GetPublishingRateLimit(ctx)
		if err != nil {
			return errorResult[rateLimitOutput](err), nil
		}
		return okResult(rateLimitOutput{
			QuotaTotal:    info.Config.QuotaTotal,
			QuotaDuration: info.Config.QuotaDuration,
			QuotaUsage:    info.QuotaUsage,
		}), nil
	})
}

// ---------------------------------------------------------------------------
// get_media_insights
// ---------------------------------------------------------------------------

type insightsInput struct {
	MediaID string   `json:"media_id"`
	Metrics []string `json:"metrics,omitempty"`
}

type insightsOutput struct {
	Data []instagram.Insight `json:"data"`
}

var defaultMetrics = []string{
	"impressions", "reach", "likes", "comments", "shares", "saved", "plays",
}

func registerGetInsights(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_media_insights",
		Description: "Retrieve engagement metrics (reach, plays, likes, saves, shares, comments) for a published post.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[insightsInput]) (*mcp.CallToolResultFor[insightsOutput], error) {
		metrics := params.Arguments.Metrics
		if len(metrics) == 0 {
			metrics = defaultMetrics
		}

		insights, err := client.GetMediaInsights(ctx, params.Arguments.MediaID, metrics)
		if err != nil {
			return errorResult[insightsOutput](err), nil
		}
		return okResult(insightsOutput{Data: insights}), nil
	})
}

// ---------------------------------------------------------------------------
// list_recent_media
// ---------------------------------------------------------------------------

type listMediaInput struct {
	Limit int `json:"limit,omitempty"`
}

type listMediaOutput struct {
	Data []instagram.MediaItem `json:"data"`
}

func registerListMedia(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_recent_media",
		Description: "List recent posts, reels, and carousels from the Instagram account.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[listMediaInput]) (*mcp.CallToolResultFor[listMediaOutput], error) {
		limit := params.Arguments.Limit
		if limit <= 0 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		media, err := client.ListRecentMedia(ctx, limit)
		if err != nil {
			return errorResult[listMediaOutput](err), nil
		}
		return okResult(listMediaOutput{Data: media}), nil
	})
}
