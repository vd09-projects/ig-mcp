package instagram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func fakeInsight(name string, v int) Insight {
	return Insight{Name: name, Values: []InsightValue{{Value: v}}}
}

func TestGetReelsReport(t *testing.T) {
	mock := &MockClient{
		ListRecentMediaFn: func(_ context.Context, limit int) ([]MediaItem, error) {
			if limit != 25 {
				t.Errorf("expected default limit 25, got %d", limit)
			}
			return []MediaItem{
				{ID: "r1", MediaProductType: ProductReels, Permalink: "p1", Timestamp: "2026-10-01T10:00:00+0000"},
				{ID: "img", MediaProductType: ProductFeed, Timestamp: "2026-10-02T10:00:00+0000"},
				{ID: "r2", MediaProductType: ProductReels, Permalink: "p2", Timestamp: "2026-10-03T10:00:00+0000", LikeCount: 7, CommentsCount: 2},
			}, nil
		},
		GetMediaInsightsFn: func(_ context.Context, id string, _ []string) ([]Insight, error) {
			switch id {
			case "r1":
				return []Insight{
					fakeInsight("views", 1000), fakeInsight("reach", 800), fakeInsight("likes", 50),
					fakeInsight("comments", 5), fakeInsight("shares", 10), fakeInsight("saved", 15),
					fakeInsight("total_interactions", 80), fakeInsight("ig_reels_avg_watch_time", 4200),
				}, nil
			case "r2":
				return nil, errors.New("media too new")
			}
			t.Errorf("unexpected insights call for %s", id)
			return nil, nil
		},
	}

	// Concurrency > 1 keeps the parallel fetch path under -race.
	out, err := GetReelsReport(context.Background(), mock, ReelsReportOptions{Limit: 25, Concurrency: 5})
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

func TestGetReelsReportSince(t *testing.T) {
	mock := &MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]MediaItem, error) {
			return []MediaItem{
				{ID: "new", MediaProductType: ProductReels, Timestamp: "2026-10-05T10:00:00+0000"},
				{ID: "old", MediaProductType: ProductReels, Timestamp: "2026-09-01T10:00:00+0000"},
			}, nil
		},
		GetMediaInsightsFn: func(_ context.Context, id string, _ []string) ([]Insight, error) {
			if id != "new" {
				t.Errorf("old reel should be filtered out, got %s", id)
			}
			return nil, nil
		},
	}

	since := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	out, err := GetReelsReport(context.Background(), mock, ReelsReportOptions{Limit: 25, Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Reels) != 1 || out.Reels[0].ID != "new" {
		t.Errorf("expected only 'new' reel, got %+v", out.Reels)
	}
}

func TestGetReelsReportAllInsightsFail(t *testing.T) {
	mock := &MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]MediaItem, error) {
			return []MediaItem{
				{ID: "r1", MediaProductType: ProductReels},
				{ID: "r2", MediaProductType: ProductReels},
			}, nil
		},
		GetMediaInsightsFn: func(context.Context, string, []string) ([]Insight, error) {
			return nil, errors.New("missing instagram_business_manage_insights permission")
		},
	}

	_, err := GetReelsReport(context.Background(), mock, ReelsReportOptions{Limit: 25})
	if err == nil || !strings.Contains(err.Error(), "all 2 reels") || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("expected systemic error, got %v", err)
	}
}

func TestGetReelsReportNoReels(t *testing.T) {
	mock := &MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]MediaItem, error) {
			return []MediaItem{{ID: "img", MediaProductType: ProductFeed}}, nil
		},
	}

	out, err := GetReelsReport(context.Background(), mock, ReelsReportOptions{Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary.ReelCount != 0 || len(out.Reels) != 0 {
		t.Errorf("expected empty result, got %+v", out)
	}
}

func TestGetReelsReportCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mock := &MockClient{
		ListRecentMediaFn: func(context.Context, int) ([]MediaItem, error) {
			cancel()
			return []MediaItem{{ID: "r1", MediaProductType: ProductReels}}, nil
		},
		GetMediaInsightsFn: func(ctx context.Context, _ string, _ []string) ([]Insight, error) {
			return nil, ctx.Err()
		},
	}

	if _, err := GetReelsReport(ctx, mock, ReelsReportOptions{Limit: 25}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
