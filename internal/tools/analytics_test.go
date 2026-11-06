package tools

import (
	"testing"
	"time"
)

func TestClampLimit(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, 25}, {-3, 25}, {40, 40}, {9999, 500}} {
		if got := clampLimit(tc.in, 25, 500); got != tc.want {
			t.Errorf("clampLimit(%d): expected %d, got %d", tc.in, tc.want, got)
		}
	}
}

func TestReelsReportOptions(t *testing.T) {
	toolOpts := Options{MaxMediaLimit: 500, ReelsDefault: 25, InsightsConcurrency: 5}

	opts, err := reelsReportOptions(reelsAnalyticsInput{Since: "2026-10-01"}, toolOpts)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Limit != 25 || opts.Concurrency != 5 || !opts.Since.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("unexpected options: %+v", opts)
	}

	for _, since := range []string{"2026-10-01T10:00:00Z", "2026-10-01T10:00:00+0000"} {
		if _, err := reelsReportOptions(reelsAnalyticsInput{Since: since}, toolOpts); err != nil {
			t.Errorf("since %q: unexpected error %v", since, err)
		}
	}

	if _, err := reelsReportOptions(reelsAnalyticsInput{Since: "last week"}, toolOpts); err == nil {
		t.Error("expected error for invalid since")
	}
}
