package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// ---------------------------------------------------------------------------
// get_reels_analytics
// ---------------------------------------------------------------------------

type reelsAnalyticsInput struct {
	Limit int    `json:"limit,omitempty"`
	Since string `json:"since,omitempty"`
}

func registerGetReelsAnalytics(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_reels_analytics",
		Description: "Fetch per-Reel analytics for the account: views, reach, likes, comments, shares, saves, " +
			"watch time and engagement rate, plus account-level totals. " +
			"limit = how many recent media items to scan (default 25, max 500); " +
			"since = optional RFC3339 or YYYY-MM-DD date to only include Reels published on/after it.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[reelsAnalyticsInput]) (*mcp.CallToolResultFor[instagram.ReelsReport], error) {
		opts, err := reelsReportOptions(params.Arguments)
		if err != nil {
			return errorResult[instagram.ReelsReport](err), nil
		}

		report, err := instagram.GetReelsReport(ctx, client, opts)
		if err != nil {
			return errorResult[instagram.ReelsReport](err), nil
		}
		return okResult(*report), nil
	})
}

func reelsReportOptions(in reelsAnalyticsInput) (instagram.ReelsReportOptions, error) {
	opts := instagram.ReelsReportOptions{Limit: clampLimit(in.Limit, 25)}
	if in.Since != "" {
		since, err := parseSince(in.Since)
		if err != nil {
			return opts, err
		}
		opts.Since = since
	}
	return opts, nil
}

func parseSince(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", instagram.TimestampLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid since %q: use YYYY-MM-DD or RFC3339", s)
}
