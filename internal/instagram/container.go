package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/vikrant/instagram-mcp/internal/graphapi"
)

// CreateReelContainer creates a media container for a Reel video.
func (c *graphClient) CreateReelContainer(ctx context.Context, p ReelParams) (string, error) {
	form := url.Values{
		"media_type": {"REELS"},
		"video_url":  {p.VideoURL},
	}
	setOptionalString(form, "caption", p.Caption)
	setOptionalString(form, "location_id", p.LocationID)
	setOptionalString(form, "cover_url", p.CoverURL)
	setOptionalString(form, "audio_name", p.AudioName)

	if p.ShareToFeed != nil {
		form.Set("share_to_feed", strconv.FormatBool(*p.ShareToFeed))
	}
	if p.ThumbOffset != nil {
		form.Set("thumb_offset", strconv.Itoa(*p.ThumbOffset))
	}
	if err := setOptionalJSON(form, "user_tags", p.UserTags); err != nil {
		return "", fmt.Errorf("marshaling user_tags: %w", err)
	}
	if err := setOptionalJSON(form, "collaborators", p.Collaborators); err != nil {
		return "", fmt.Errorf("marshaling collaborators: %w", err)
	}

	raw, err := c.api.Post(ctx, "/"+c.accountID+"/media", form)
	if err != nil {
		return "", fmt.Errorf("creating reel container: %w", err)
	}
	return graphapi.ExtractID(raw)
}

// CreateImageContainer creates a media container for a single image post.
func (c *graphClient) CreateImageContainer(ctx context.Context, p ImageParams) (string, error) {
	form := url.Values{
		"image_url": {p.ImageURL},
	}
	setOptionalString(form, "caption", p.Caption)
	setOptionalString(form, "location_id", p.LocationID)
	setOptionalString(form, "alt_text", p.AltText)

	if err := setOptionalJSON(form, "user_tags", p.UserTags); err != nil {
		return "", fmt.Errorf("marshaling user_tags: %w", err)
	}

	raw, err := c.api.Post(ctx, "/"+c.accountID+"/media", form)
	if err != nil {
		return "", fmt.Errorf("creating image container: %w", err)
	}
	return graphapi.ExtractID(raw)
}

// CreateCarouselContainer creates a carousel container from child container IDs.
func (c *graphClient) CreateCarouselContainer(ctx context.Context, p CarouselParams) (string, error) {
	form := url.Values{
		"media_type": {"CAROUSEL"},
	}
	for _, id := range p.Children {
		form.Add("children", id)
	}
	setOptionalString(form, "caption", p.Caption)
	setOptionalString(form, "location_id", p.LocationID)

	if err := setOptionalJSON(form, "collaborators", p.Collaborators); err != nil {
		return "", fmt.Errorf("marshaling collaborators: %w", err)
	}

	raw, err := c.api.Post(ctx, "/"+c.accountID+"/media", form)
	if err != nil {
		return "", fmt.Errorf("creating carousel container: %w", err)
	}
	return graphapi.ExtractID(raw)
}

// ---------------------------------------------------------------------------
// Form helpers — reduce repetitive nil/empty checks
// ---------------------------------------------------------------------------

func setOptionalString(form url.Values, key, val string) {
	if val != "" {
		form.Set(key, val)
	}
}

func setOptionalJSON[T any](form url.Values, key string, val []T) error {
	if len(val) == 0 {
		return nil
	}
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	form.Set(key, string(b))
	return nil
}
