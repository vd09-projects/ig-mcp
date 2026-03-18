// Package instagram provides a high-level client for Instagram Graph API
// content publishing (Reels, images, carousels) and insights.
//
// It is built on top of the generic graphapi package and adds
// Instagram-specific form encoding, container polling, and response parsing.
package instagram

import (
	"context"
	"time"

	"github.com/vikrant/instagram-mcp/internal/graphapi"
)

// Client defines the operations the Instagram MCP server needs.
// All methods accept context for cancellation and timeout support.
type Client interface {
	// Publishing
	CreateReelContainer(ctx context.Context, p ReelParams) (containerID string, err error)
	CreateImageContainer(ctx context.Context, p ImageParams) (containerID string, err error)
	CreateCarouselContainer(ctx context.Context, p CarouselParams) (containerID string, err error)

	// Container lifecycle
	GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatusResult, error)
	WaitForContainer(ctx context.Context, containerID string) (*ContainerStatusResult, error)
	Publish(ctx context.Context, containerID string) (*PublishResult, error)

	// Queries
	GetMediaInsights(ctx context.Context, mediaID string, metrics []string) ([]Insight, error)
	GetPublishingRateLimit(ctx context.Context) (*RateLimitInfo, error)
	ListRecentMedia(ctx context.Context, limit int) ([]MediaItem, error)
}

// graphClient is the production implementation of Client.
type graphClient struct {
	api             *graphapi.Client
	accountID       string
	pollInterval    time.Duration
	pollMaxAttempts int
}

// NewClient creates a production Instagram client backed by the Graph API.
func NewClient(
	api *graphapi.Client,
	accountID string,
	pollInterval time.Duration,
	pollMaxAttempts int,
) Client {
	return &graphClient{
		api:             api,
		accountID:       accountID,
		pollInterval:    pollInterval,
		pollMaxAttempts: pollMaxAttempts,
	}
}
