package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vikrant/instagram-mcp/internal/instagram"
)

func insight(name string, v int) instagram.Insight {
	return instagram.Insight{Name: name, Values: []instagram.InsightValue{{Value: v}}}
}

func TestBuildReelsAnalytics(t *testing.T) {
	mock := &instagram.MockClient{
		ListRecentMediaFn: func(_ context.Context, limit int) ([]instagram.MediaItem, error) {
			if limit != 25 {
				t.Errorf("expected default limit 25, got %d", limit)
			}
			return []instagram.MediaItem{
				{ID: "r1", MediaProductType: instagram.ProductReels, Permalink: "p1", Timestamp: "2026-10-01T10:00:00+0000"},
				{ID: "img", MediaProductType: instagram.ProductFeed, Timestamp: "2026-10-02T10:00:00+0000"},
				{ID: "r2", MediaProductType: instagram.ProductReels, Permalink: "p2", Timestamp: "2026-10-03T10:00:00+0000", LikeCount: 7, CommentsCount: 2},
			}, nil
		},
		GetMediaInsightsFn: func(_ context.Context, id string, _ []string) ([]instagram.Insight, error) {
			switch id {
			case "r1":
				return []instagram.Insight{
					insight("views", 1000), insight("reach", 800), insight("likes", 50),
					insight("comments", 5), insight("shares", 10), insight("saved", 15),
					insight("total_interactions", 80), insight("ig_reels_avg_watch_time", 4200),
				}, nil
			case "r2":
				return nil, errors.New("media too new")
			}
			t.Fatalf("unexpected insights call for %s", id)
			return nil, nil
		},
	}

	out, err := buildReelsAnalytics(context.Background(), mock, reelsAnalyticsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Reels) != 2 {
		t.Fatalf("expected 2 reels, got %d", len(out.Reels))
	}

	r1 := out.Reels[0]
	if r1.Views != 1000 || r1.Shares != 10 || r1.AvgWatchTimeMs != 4200 || r1.EngagementRatePct != 10 {
		t.Errorf("unexpected r1 stats: %+v", r1)
	}

	r2 := out.Reels[1]
	if r2.InsightsUnavailable == "" || r2.Likes != 7 || r2.Comments != 2 {
		t.Errorf("expected r2 to fall back to public counts: %+v", r2)
	}

	s := out.Summary
	// r2 has no insights, so it is excluded from totals and averages.
	if s.ReelCount != 2 || s.ReelsWithoutInsights != 1 || s.TotalViews != 1000 || s.TotalLikes != 50 ||
		s.AvgViewsPerReel != 1000 || s.AvgEngagementRatePct != 10 || s.TopReelByViews != "p1" {
		t.Errorf("unexpected summary: %+v", s)
	}
}

func TestBuildReelsAnalyticsSince(t *testing.T) {
	mock := &instagram.MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]instagram.MediaItem, error) {
			return []instagram.MediaItem{
				{ID: "new", MediaProductType: instagram.ProductReels, Timestamp: "2026-10-05T10:00:00+0000"},
				{ID: "old", MediaProductType: instagram.ProductReels, Timestamp: "2026-09-01T10:00:00+0000"},
			}, nil
		},
		GetMediaInsightsFn: func(_ context.Context, id string, _ []string) ([]instagram.Insight, error) {
			if id != "new" {
				t.Errorf("old reel should be filtered out, got %s", id)
			}
			return nil, nil
		},
	}

	out, err := buildReelsAnalytics(context.Background(), mock, reelsAnalyticsInput{Since: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Reels) != 1 || out.Reels[0].ID != "new" {
		t.Errorf("expected only 'new' reel, got %+v", out.Reels)
	}

	if _, err := buildReelsAnalytics(context.Background(), mock, reelsAnalyticsInput{Since: "last week"}); err == nil {
		t.Error("expected error for invalid since")
	}
}

func TestBuildReelsAnalyticsAllInsightsFail(t *testing.T) {
	mock := &instagram.MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]instagram.MediaItem, error) {
			return []instagram.MediaItem{
				{ID: "r1", MediaProductType: instagram.ProductReels},
				{ID: "r2", MediaProductType: instagram.ProductReels},
			}, nil
		},
		GetMediaInsightsFn: func(context.Context, string, []string) ([]instagram.Insight, error) {
			return nil, errors.New("missing instagram_business_manage_insights permission")
		},
	}

	_, err := buildReelsAnalytics(context.Background(), mock, reelsAnalyticsInput{})
	if err == nil || !strings.Contains(err.Error(), "all 2 reels") || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("expected systemic error, got %v", err)
	}
}

func TestBuildReelsAnalyticsNoReels(t *testing.T) {
	mock := &instagram.MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]instagram.MediaItem, error) {
			return []instagram.MediaItem{{ID: "img", MediaProductType: instagram.ProductFeed}}, nil
		},
	}

	out, err := buildReelsAnalytics(context.Background(), mock, reelsAnalyticsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary.ReelCount != 0 || len(out.Reels) != 0 {
		t.Errorf("expected empty result, got %+v", out)
	}
}

func TestBuildReelsAnalyticsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mock := &instagram.MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]instagram.MediaItem, error) {
			cancel()
			return []instagram.MediaItem{{ID: "r1", MediaProductType: instagram.ProductReels}}, nil
		},
		GetMediaInsightsFn: func(ctx context.Context, _ string, _ []string) ([]instagram.Insight, error) {
			return nil, ctx.Err()
		},
	}

	if _, err := buildReelsAnalytics(ctx, mock, reelsAnalyticsInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestBuildReelsAnalyticsLimitClamp(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, 25}, {-3, 25}, {40, 40}, {9999, maxMediaLimit}} {
		var got int
		mock := &instagram.MockClient{
			ListRecentMediaFn: func(_ context.Context, limit int) ([]instagram.MediaItem, error) {
				got = limit
				return nil, nil
			},
		}
		if _, err := buildReelsAnalytics(context.Background(), mock, reelsAnalyticsInput{Limit: tc.in}); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("limit %d: expected %d, got %d", tc.in, tc.want, got)
		}
	}
}
