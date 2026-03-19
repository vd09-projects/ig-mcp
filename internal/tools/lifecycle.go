package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// ---------------------------------------------------------------------------
// check_container_status
// ---------------------------------------------------------------------------

type checkStatusInput struct {
	ContainerID string `json:"container_id"`
}

type checkStatusOutput struct {
	ID         string `json:"id"`
	StatusCode string `json:"status_code"`
	Status     string `json:"status,omitempty"`
}

func registerCheckContainerStatus(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_container_status",
		Description: "Check the processing status of a media container (IN_PROGRESS, FINISHED, ERROR, EXPIRED).",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[checkStatusInput]) (*mcp.CallToolResultFor[checkStatusOutput], error) {
		status, err := client.GetContainerStatus(ctx, params.Arguments.ContainerID)
		if err != nil {
			return errorResult[checkStatusOutput](err), nil
		}
		return okResult(checkStatusOutput{
			ID:         status.ID,
			StatusCode: string(status.StatusCode),
			Status:     status.Status,
		}), nil
	})
}

// ---------------------------------------------------------------------------
// publish_media
// ---------------------------------------------------------------------------

type publishInput struct {
	ContainerID string `json:"container_id"`
}

type publishOutput struct {
	Status  string `json:"status"`
	MediaID string `json:"media_id"`
	Message string `json:"message"`
}

func registerPublishMedia(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "publish_media",
		Description: "Publish a media container that has finished processing.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[publishInput]) (*mcp.CallToolResultFor[publishOutput], error) {
		pub, err := client.Publish(ctx, params.Arguments.ContainerID)
		if err != nil {
			return errorResult[publishOutput](err), nil
		}
		return okResult(publishOutput{
			Status:  "published",
			MediaID: pub.ID,
			Message: "Media published successfully!",
		}), nil
	})
}
