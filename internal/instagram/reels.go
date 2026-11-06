package instagram

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// ReelMetrics are the insight metrics requested for every Reel.
// All are valid for REELS media; ig_reels_* metrics fail on other media types.
var ReelMetrics = []string{
	"views", "reach", "likes", "comments", "shares", "saved", "total_interactions",
	"ig_reels_avg_watch_time", "ig_reels_video_view_total_time",
}

// ReelsReportOptions selects which Reels a report covers.
type ReelsReportOptions struct {
	// Limit is how many recent media items to scan; non-Reels are skipped.
	Limit int
	// Since, when non-zero, drops Reels published before it. Reels whose
	// timestamp can't be parsed are kept rather than silently dropped.
	Since time.Time
	// Concurrency bounds parallel insight requests to stay well under Graph
	// API rate limits. Values below 1 fetch sequentially.
	Concurrency int
}

// ReelStats holds the analytics for a single Reel.
type ReelStats struct {
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

// HasInsights reports whether insights were fetched for the Reel.
func (s ReelStats) HasInsights() bool {
	return s.InsightsUnavailable == ""
}

// ReelsSummary aggregates only Reels whose insights were fetched; Reels without
// insights are counted in ReelsWithoutInsights so they don't skew the averages.
type ReelsSummary struct {
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

// ReelsReport is the per-Reel analytics plus their summary.
type ReelsReport struct {
	Summary ReelsSummary `json:"summary"`
	Reels   []ReelStats  `json:"reels"`
}

// GetReelsReport lists recent media, keeps Reels, and fetches insights for each.
//
// A per-Reel insights failure (e.g. a Reel too new to have data) is recorded on
// that Reel rather than failing the report. If every Reel fails, the cause is
// systemic (missing scope, rate limit) and an error is returned instead of a
// summary of zeros.
func GetReelsReport(ctx context.Context, c Client, opts ReelsReportOptions) (*ReelsReport, error) {
	media, err := c.ListRecentMedia(ctx, opts.Limit)
	if err != nil {
		return nil, err
	}

	stats := fetchAllReelStats(ctx, c, filterReels(media, opts.Since), opts.Concurrency)
	if err := checkInsightsFetched(ctx, stats); err != nil {
		return nil, err
	}
	return &ReelsReport{Summary: summarize(stats), Reels: stats}, nil
}

func filterReels(media []MediaItem, since time.Time) []MediaItem {
	var reels []MediaItem
	for _, m := range media {
		if !m.IsReel() {
			continue
		}
		if !since.IsZero() {
			if ts, err := m.PublishedAt(); err == nil && ts.Before(since) {
				continue
			}
		}
		reels = append(reels, m)
	}
	return reels
}

// fetchAllReelStats fetches insights for each Reel, at most concurrency at a
// time. Results keep the order of reels.
func fetchAllReelStats(ctx context.Context, c Client, reels []MediaItem, concurrency int) []ReelStats {
	stats := make([]ReelStats, len(reels))
	sem := make(chan struct{}, max(concurrency, 1))
	var wg sync.WaitGroup
	for i, r := range reels {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				stats[i] = fetchReelStats(ctx, c, r)
			case <-ctx.Done():
				stats[i] = ReelStats{ID: r.ID, InsightsUnavailable: ctx.Err().Error()}
			}
		}()
	}
	wg.Wait()
	return stats
}

func fetchReelStats(ctx context.Context, c Client, r MediaItem) ReelStats {
	s := ReelStats{
		ID:        r.ID,
		Caption:   r.Caption,
		Permalink: r.Permalink,
		Timestamp: r.Timestamp,
		// Fall back to the public counts on the media node if insights fail.
		Likes:    r.LikeCount,
		Comments: r.CommentsCount,
	}

	insights, err := c.GetMediaInsights(ctx, r.ID, ReelMetrics)
	if err != nil {
		s.InsightsUnavailable = err.Error()
		return s
	}

	v := insightValues(insights)
	s.Views = v["views"]
	s.Reach = v["reach"]
	s.Likes = v["likes"]
	s.Comments = v["comments"]
	s.Shares = v["shares"]
	s.Saved = v["saved"]
	s.TotalInteractions = v["total_interactions"]
	s.AvgWatchTimeMs = v["ig_reels_avg_watch_time"]
	s.TotalWatchTimeMs = v["ig_reels_video_view_total_time"]
	if s.Reach > 0 {
		s.EngagementRatePct = round2(float64(s.TotalInteractions) / float64(s.Reach) * 100)
	}
	return s
}

// insightValues maps each metric name to its first value. Metrics with no
// values are omitted, so lookups for them return 0.
func insightValues(insights []Insight) map[string]int {
	values := make(map[string]int, len(insights))
	for _, in := range insights {
		if len(in.Values) > 0 {
			values[in.Name] = in.Values[0].Value
		}
	}
	return values
}

// checkInsightsFetched returns an error if ctx was cancelled or if there were
// Reels and none had insights.
func checkInsightsFetched(ctx context.Context, stats []ReelStats) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(stats) == 0 {
		return nil
	}
	for _, s := range stats {
		if s.HasInsights() {
			return nil
		}
	}
	return fmt.Errorf("insights unavailable for all %d reels: %s", len(stats), stats[0].InsightsUnavailable)
}

func summarize(stats []ReelStats) ReelsSummary {
	sum := ReelsSummary{ReelCount: len(stats)}
	var rateTotal float64
	var rated, topViews int
	for _, s := range stats {
		if !s.HasInsights() {
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

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
