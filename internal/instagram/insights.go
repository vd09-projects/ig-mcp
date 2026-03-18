package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetMediaInsights retrieves engagement metrics for a published post or Reel.
func (c *graphClient) GetMediaInsights(ctx context.Context, mediaID string, metrics []string) ([]Insight, error) {
	raw, err := c.api.Get(ctx, "/"+mediaID+"/insights", url.Values{
		"metric": {strings.Join(metrics, ",")},
	})
	if err != nil {
		return nil, fmt.Errorf("getting media insights: %w", err)
	}

	var envelope struct {
		Data []Insight `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parsing insights: %w", err)
	}
	return envelope.Data, nil
}

// GetPublishingRateLimit checks how many posts remain in the 24-hour window.
func (c *graphClient) GetPublishingRateLimit(ctx context.Context) (*RateLimitInfo, error) {
	raw, err := c.api.Get(ctx, "/"+c.accountID+"/content_publishing_limit", url.Values{
		"fields": {"config,quota_usage"},
	})
	if err != nil {
		return nil, fmt.Errorf("getting rate limit: %w", err)
	}

	var result RateLimitInfo
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing rate limit: %w", err)
	}
	return &result, nil
}

// ListRecentMedia returns the most recent media items from the account.
func (c *graphClient) ListRecentMedia(ctx context.Context, limit int) ([]MediaItem, error) {
	raw, err := c.api.Get(ctx, "/"+c.accountID+"/media", url.Values{
		"fields": {"id,caption,media_type,media_url,permalink,timestamp,thumbnail_url"},
		"limit":  {strconv.Itoa(limit)},
	})
	if err != nil {
		return nil, fmt.Errorf("listing media: %w", err)
	}

	var envelope struct {
		Data []MediaItem `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parsing media list: %w", err)
	}
	return envelope.Data, nil
}
