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

func registerGetReelsAnalytics(server *mcp.Server, client instagram.Client, toolOpts Options) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_reels_analytics",
		Description: "Fetch per-Reel analytics for the account: views, reach, likes, comments, shares, saves, " +
			"watch time and engagement rate, plus account-level totals. " +
			fmt.Sprintf("limit = how many recent media items to scan (default %d, max %d); ", toolOpts.ReelsDefault, toolOpts.MaxMediaLimit) +
			"since = optional RFC3339 or YYYY-MM-DD date to only include Reels published on/after it.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[reelsAnalyticsInput]) (*mcp.CallToolResultFor[instagram.ReelsReport], error) {
		reportOpts, err := reelsReportOptions(params.Arguments, toolOpts)
		if err != nil {
			return errorResult[instagram.ReelsReport](err), nil
		}

		report, err := instagram.GetReelsReport(ctx, client, reportOpts)
		if err != nil {
			return errorResult[instagram.ReelsReport](err), nil
		}
		return okResult(*report), nil
	})
}

func reelsReportOptions(in reelsAnalyticsInput, toolOpts Options) (instagram.ReelsReportOptions, error) {
	reportOpts := instagram.ReelsReportOptions{
		Limit:       clampLimit(in.Limit, toolOpts.ReelsDefault, toolOpts.MaxMediaLimit),
		Concurrency: toolOpts.InsightsConcurrency,
	}
	if in.Since != "" {
		since, err := parseSince(in.Since)
		if err != nil {
			return reportOpts, err
		}
		reportOpts.Since = since
	}
	return reportOpts, nil
}

func parseSince(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", instagram.TimestampLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid since %q: use YYYY-MM-DD or RFC3339", s)
}
