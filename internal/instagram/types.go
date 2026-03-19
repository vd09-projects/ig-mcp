package instagram

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// ContainerStatus represents the processing state of a media container.
type ContainerStatus string

const (
	StatusExpired    ContainerStatus = "EXPIRED"
	StatusError      ContainerStatus = "ERROR"
	StatusFinished   ContainerStatus = "FINISHED"
	StatusInProgress ContainerStatus = "IN_PROGRESS"
	StatusPublished  ContainerStatus = "PUBLISHED"
)

// ---------------------------------------------------------------------------
// Request params
// ---------------------------------------------------------------------------

// UserTag identifies a person to tag in a media post.
type UserTag struct {
	Username string  `json:"username"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
}

// ReelParams holds every parameter accepted when creating a Reel container.
type ReelParams struct {
	VideoURL      string    `json:"video_url"`
	Caption       string    `json:"caption,omitempty"`
	ShareToFeed   *bool     `json:"share_to_feed,omitempty"`
	ThumbOffset   *int      `json:"thumb_offset,omitempty"`
	LocationID    string    `json:"location_id,omitempty"`
	CoverURL      string    `json:"cover_url,omitempty"`
	UserTags      []UserTag `json:"user_tags,omitempty"`
	Collaborators []string  `json:"collaborators,omitempty"`
	AudioName     string    `json:"audio_name,omitempty"`
}

// ImageParams holds parameters for a single image post.
type ImageParams struct {
	ImageURL   string    `json:"image_url"`
	Caption    string    `json:"caption,omitempty"`
	LocationID string    `json:"location_id,omitempty"`
	UserTags   []UserTag `json:"user_tags,omitempty"`
	AltText    string    `json:"alt_text,omitempty"`
}

// CarouselParams holds parameters for a carousel (multi-media) post.
type CarouselParams struct {
	Children      []string `json:"children"`
	Caption       string   `json:"caption,omitempty"`
	LocationID    string   `json:"location_id,omitempty"`
	Collaborators []string `json:"collaborators,omitempty"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// ContainerStatusResult is the response from a container status check.
type ContainerStatusResult struct {
	ID           string          `json:"id"`
	StatusCode   ContainerStatus `json:"status_code"`
	Status       string          `json:"status,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

// PublishResult is the response from publishing a media container.
type PublishResult struct {
	ID string `json:"id"`
}

// MediaItem represents a single media object returned by the account media list.
type MediaItem struct {
	ID           string `json:"id"`
	Caption      string `json:"caption,omitempty"`
	MediaType    string `json:"media_type,omitempty"`
	MediaURL     string `json:"media_url,omitempty"`
	Permalink    string `json:"permalink,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// InsightValue is a single data point within an insight metric.
type InsightValue struct {
	Value int `json:"value"`
}

// Insight represents a single engagement metric.
type Insight struct {
	Name        string         `json:"name"`
	Period      string         `json:"period"`
	Values      []InsightValue `json:"values"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
}

// RateLimitConfig holds the quota configuration.
type RateLimitConfig struct {
	QuotaTotal    int `json:"quota_total"`
	QuotaDuration int `json:"quota_duration"`
}

// RateLimitInfo represents the publishing rate limit status.
type RateLimitInfo struct {
	Config     RateLimitConfig `json:"config"`
	QuotaUsage int             `json:"quota_usage"`
}
