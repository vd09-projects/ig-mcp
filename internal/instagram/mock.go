package instagram

import "context"

// MockClient implements Client for unit testing. Set the function fields
// to control behavior per-test; any nil function will panic, making
// unexpected calls immediately visible.
type MockClient struct {
	CreateReelContainerFn     func(ctx context.Context, p ReelParams) (string, error)
	CreateImageContainerFn    func(ctx context.Context, p ImageParams) (string, error)
	CreateCarouselContainerFn func(ctx context.Context, p CarouselParams) (string, error)
	GetContainerStatusFn      func(ctx context.Context, containerID string) (*ContainerStatusResult, error)
	WaitForContainerFn        func(ctx context.Context, containerID string) (*ContainerStatusResult, error)
	PublishFn                 func(ctx context.Context, containerID string) (*PublishResult, error)
	GetMediaInsightsFn        func(ctx context.Context, mediaID string, metrics []string) ([]Insight, error)
	GetPublishingRateLimitFn  func(ctx context.Context) (*RateLimitInfo, error)
	ListRecentMediaFn         func(ctx context.Context, limit int) ([]MediaItem, error)
}

var _ Client = (*MockClient)(nil) // compile-time interface check

func (m *MockClient) CreateReelContainer(ctx context.Context, p ReelParams) (string, error) {
	return m.CreateReelContainerFn(ctx, p)
}

func (m *MockClient) CreateImageContainer(ctx context.Context, p ImageParams) (string, error) {
	return m.CreateImageContainerFn(ctx, p)
}

func (m *MockClient) CreateCarouselContainer(ctx context.Context, p CarouselParams) (string, error) {
	return m.CreateCarouselContainerFn(ctx, p)
}

func (m *MockClient) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatusResult, error) {
	return m.GetContainerStatusFn(ctx, containerID)
}

func (m *MockClient) WaitForContainer(ctx context.Context, containerID string) (*ContainerStatusResult, error) {
	return m.WaitForContainerFn(ctx, containerID)
}

func (m *MockClient) Publish(ctx context.Context, containerID string) (*PublishResult, error) {
	return m.PublishFn(ctx, containerID)
}

func (m *MockClient) GetMediaInsights(ctx context.Context, mediaID string, metrics []string) ([]Insight, error) {
	return m.GetMediaInsightsFn(ctx, mediaID, metrics)
}

func (m *MockClient) GetPublishingRateLimit(ctx context.Context) (*RateLimitInfo, error) {
	return m.GetPublishingRateLimitFn(ctx)
}

func (m *MockClient) ListRecentMedia(ctx context.Context, limit int) ([]MediaItem, error) {
	return m.ListRecentMediaFn(ctx, limit)
}
