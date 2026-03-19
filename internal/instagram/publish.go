package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/vikrant/instagram-mcp/internal/graphapi"
)

// GetContainerStatus checks the processing status of a media container.
func (c *graphClient) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatusResult, error) {
	raw, err := c.api.Get(ctx, "/"+containerID, url.Values{
		"fields": {"id,status_code,status,error_message"},
	})
	if err != nil {
		return nil, fmt.Errorf("getting container status: %w", err)
	}

	var result ContainerStatusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing container status: %w", err)
	}
	return &result, nil
}

// WaitForContainer polls container status until FINISHED, or returns an error
// on failure/timeout. It respects context cancellation.
func (c *graphClient) WaitForContainer(ctx context.Context, containerID string) (*ContainerStatusResult, error) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for attempt := range c.pollMaxAttempts {
		status, err := c.GetContainerStatus(ctx, containerID)
		if err != nil {
			return nil, err
		}

		switch status.StatusCode {
		case StatusFinished:
			return status, nil
		case StatusError, StatusExpired:
			detail := status.Status
			if status.ErrorMessage != "" {
				detail = status.ErrorMessage
			}
			return nil, fmt.Errorf("container %s failed (%s): %s",
				containerID, status.StatusCode, detail)
		case StatusInProgress:
			// continue polling
		default:
			return nil, fmt.Errorf("unexpected container status: %s", status.StatusCode)
		}

		if attempt < c.pollMaxAttempts-1 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled while waiting: %w", ctx.Err())
			case <-ticker.C:
			}
		}
	}

	return nil, fmt.Errorf("container %s did not finish after %d attempts",
		containerID, c.pollMaxAttempts)
}

// Publish publishes a media container that has finished processing.
func (c *graphClient) Publish(ctx context.Context, containerID string) (*PublishResult, error) {
	raw, err := c.api.Post(ctx, "/"+c.accountID+"/media_publish", url.Values{
		"creation_id": {containerID},
	})
	if err != nil {
		return nil, fmt.Errorf("publishing media: %w", err)
	}

	id, err := graphapi.ExtractID(raw)
	if err != nil {
		return nil, err
	}
	return &PublishResult{ID: id}, nil
}
