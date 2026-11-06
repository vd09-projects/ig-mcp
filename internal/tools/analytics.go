package tools

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// ---------------------------------------------------------------------------
// get_reels_analytics
// ---------------------------------------------------------------------------

// reelMetrics are requested for every Reel. All are valid for REELS media.
var reelMetrics = []string{
	"views", "reach", "likes", "comments", "shares", "saved", "total_interactions",
	"ig_reels_avg_watch_time", "ig_reels_video_view_total_time",
}

// insightsConcurrency bounds parallel insight requests to stay well under
// Graph API rate limits.
const insightsConcurrency = 5

type reelsAnalyticsInput struct {
	Limit int    `json:"limit,omitempty"`
	Since string `json:"since,omitempty"`
}

type reelStats struct {
	ID                  string  `json:"id"`
	Caption             string  `json:"caption,omitempty"`
	Permalink           string  `json:"permalink,omitempty"`
	Timestamp           string  `json:"timestamp,omitempty"`
	Views               int     `json:"views"`
	Reach               int     `json:"reach"`
	Likes               int     `json:"likes"`
	Comments            int     `json:"comments"`
	Shares              int     `json:"shares"`
	Saved               int     `json:"saved"`
	TotalInteractions   int     `json:"total_interactions"`
	AvgWatchTimeMs      int     `json:"avg_watch_time_ms"`
	TotalWatchTimeMs    int     `json:"total_watch_time_ms"`
	EngagementRatePct   float64 `json:"engagement_rate_pct"` // total_interactions / reach * 100
	InsightsUnavailable string  `json:"insights_unavailable,omitempty"`
}

// reelsSummary aggregates only Reels whose insights were fetched; Reels without
// insights are counted in ReelsWithoutInsights so they don't skew the averages.
type reelsSummary struct {
	ReelCount            int     `json:"reel_count"`
	ReelsWithoutInsights int     `json:"reels_without_insights"`
	TotalViews           int     `json:"total_views"`
	TotalReach           int     `json:"total_reach"`
	TotalLikes           int     `json:"total_likes"`
	TotalComments        int     `json:"total_comments"`
	TotalShares          int     `json:"total_shares"`
	TotalSaved           int     `json:"total_saved"`
	AvgViewsPerReel      float64 `json:"avg_views_per_reel"`
	// AvgEngagementRatePct is the unweighted mean of per-Reel engagement rates.
	AvgEngagementRatePct float64 `json:"avg_engagement_rate_pct"`
	TopReelByViews       string  `json:"top_reel_by_views,omitempty"`
}

type reelsAnalyticsOutput struct {
	Summary reelsSummary `json:"summary"`
	Reels   []reelStats  `json:"reels"`
}

func registerGetReelsAnalytics(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_reels_analytics",
		Description: "Fetch per-Reel analytics for the account: views, reach, likes, comments, shares, saves, " +
			"watch time and engagement rate, plus account-level totals. " +
			"limit = how many recent media items to scan (default 25, max 500); " +
			"since = optional RFC3339 or YYYY-MM-DD date to only include Reels published on/after it.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[reelsAnalyticsInput]) (*mcp.CallToolResultFor[reelsAnalyticsOutput], error) {
		out, err := buildReelsAnalytics(ctx, client, params.Arguments)
		if err != nil {
			return errorResult[reelsAnalyticsOutput](err), nil
		}
		return okResult(*out), nil
	})
}

// buildReelsAnalytics lists recent media, keeps Reels, and fetches insights for
// each one concurrently. A per-Reel insights failure (e.g. a Reel too new to
// have data) is recorded on that Reel rather than failing the whole call, but
// if every Reel fails the cause is systemic (missing scope, rate limit) and
// the first error is returned instead of a summary of zeros.
func buildReelsAnalytics(ctx context.Context, client instagram.Client, in reelsAnalyticsInput) (*reelsAnalyticsOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > maxMediaLimit {
		limit = maxMediaLimit
	}

	var since time.Time
	if in.Since != "" {
		t, err := parseSince(in.Since)
		if err != nil {
			return nil, err
		}
		since = t
	}

	media, err := client.ListRecentMedia(ctx, limit)
	if err != nil {
		return nil, err
	}

	var reels []instagram.MediaItem
	for _, m := range media {
		if !m.IsReel() {
			continue
		}
		if !since.IsZero() {
			if ts, err := time.Parse(igTimeLayout, m.Timestamp); err == nil && ts.Before(since) {
				continue
			}
		}
		reels = append(reels, m)
	}

	stats := make([]reelStats, len(reels))
	sem := make(chan struct{}, insightsConcurrency)
	var wg sync.WaitGroup
	for i, r := range reels {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				stats[i] = fetchReelStats(ctx, client, r)
			case <-ctx.Done():
				stats[i] = reelStats{ID: r.ID, InsightsUnavailable: ctx.Err().Error()}
			}
		}()
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := allFailed(stats); err != nil {
		return nil, err
	}
	return &reelsAnalyticsOutput{Summary: summarizeReels(stats), Reels: stats}, nil
}

// allFailed returns an error when there were Reels and none had insights.
func allFailed(stats []reelStats) error {
	if len(stats) == 0 {
		return nil
	}
	for _, s := range stats {
		if s.InsightsUnavailable == "" {
			return nil
		}
	}
	return fmt.Errorf("insights unavailable for all %d reels: %s", len(stats), stats[0].InsightsUnavailable)
}

func fetchReelStats(ctx context.Context, client instagram.Client, r instagram.MediaItem) reelStats {
	s := reelStats{
		ID:        r.ID,
		Caption:   r.Caption,
		Permalink: r.Permalink,
		Timestamp: r.Timestamp,
		// Fall back to the public counts on the media node if insights fail.
		Likes:    r.LikeCount,
		Comments: r.CommentsCount,
	}

	insights, err := client.GetMediaInsights(ctx, r.ID, reelMetrics)
	if err != nil {
		s.InsightsUnavailable = err.Error()
		return s
	}

	for _, metric := range insights {
		if len(metric.Values) == 0 {
			continue
		}
		v := metric.Values[0].Value
		switch metric.Name {
		case "views":
			s.Views = v
		case "reach":
			s.Reach = v
		case "likes":
			s.Likes = v
		case "comments":
			s.Comments = v
		case "shares":
			s.Shares = v
		case "saved":
			s.Saved = v
		case "total_interactions":
			s.TotalInteractions = v
		case "ig_reels_avg_watch_time":
			s.AvgWatchTimeMs = v
		case "ig_reels_video_view_total_time":
			s.TotalWatchTimeMs = v
		}
	}
	if s.Reach > 0 {
		s.EngagementRatePct = round2(float64(s.TotalInteractions) / float64(s.Reach) * 100)
	}
	return s
}

func summarizeReels(stats []reelStats) reelsSummary {
	sum := reelsSummary{ReelCount: len(stats)}
	var rateTotal float64
	var rated, topViews int
	for _, s := range stats {
		if s.InsightsUnavailable != "" {
			sum.ReelsWithoutInsights++
			continue
		}
		sum.TotalViews += s.Views
		sum.TotalReach += s.Reach
		sum.TotalLikes += s.Likes
		sum.TotalComments += s.Comments
		sum.TotalShares += s.Shares
		sum.TotalSaved += s.Saved
		if s.Reach > 0 {
			rateTotal += s.EngagementRatePct
			rated++
		}
		if s.Views > topViews {
			topViews = s.Views
			sum.TopReelByViews = s.Permalink
		}
	}
	if withInsights := sum.ReelCount - sum.ReelsWithoutInsights; withInsights > 0 {
		sum.AvgViewsPerReel = round2(float64(sum.TotalViews) / float64(withInsights))
	}
	if rated > 0 {
		sum.AvgEngagementRatePct = round2(rateTotal / float64(rated))
	}
	return sum
}

// igTimeLayout matches Graph API timestamps, e.g. 2025-01-15T10:30:00+0000.
const igTimeLayout = "2006-01-02T15:04:05-0700"

func parseSince(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", igTimeLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid since %q: use YYYY-MM-DD or RFC3339", s)
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
