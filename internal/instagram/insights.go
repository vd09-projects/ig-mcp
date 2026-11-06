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

// mediaPageSize is the largest page the media edge reliably returns.
const mediaPageSize = 100

// ListRecentMedia returns up to limit of the most recent media items from the
// account, newest first, following pagination cursors as needed.
func (c *graphClient) ListRecentMedia(ctx context.Context, limit int) ([]MediaItem, error) {
	var items []MediaItem
	after := ""
	for len(items) < limit {
		params := url.Values{
			"fields": {"id,caption,media_type,media_product_type,media_url,permalink,timestamp,thumbnail_url,like_count,comments_count"},
			"limit":  {strconv.Itoa(min(limit-len(items), mediaPageSize))},
		}
		if after != "" {
			params.Set("after", after)
		}

		raw, err := c.api.Get(ctx, "/"+c.accountID+"/media", params)
		if err != nil {
			return nil, fmt.Errorf("listing media: %w", err)
		}

		var envelope struct {
			Data   []MediaItem `json:"data"`
			Paging struct {
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("parsing media list: %w", err)
		}
		items = append(items, envelope.Data...)

		if envelope.Paging.Next == "" || envelope.Paging.Cursors.After == "" || len(envelope.Data) == 0 {
			break
		}
		after = envelope.Paging.Cursors.After
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
