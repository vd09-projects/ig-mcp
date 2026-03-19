package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/hosting"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// ---------------------------------------------------------------------------
// upload_reel
// ---------------------------------------------------------------------------

type uploadReelInput struct {
	VideoURL          string              `json:"video_url,omitempty"`
	VideoPath         string              `json:"video_path,omitempty"`
	Caption           string              `json:"caption,omitempty"`
	ShareToFeed       *bool               `json:"share_to_feed,omitempty"`
	ThumbOffset       *int                `json:"thumb_offset,omitempty"`
	LocationID        string              `json:"location_id,omitempty"`
	CoverURL          string              `json:"cover_url,omitempty"`
	UserTags          []instagram.UserTag `json:"user_tags,omitempty"`
	Collaborators     []string            `json:"collaborators,omitempty"`
	AudioName         string              `json:"audio_name,omitempty"`
	WaitForProcessing *bool               `json:"wait_for_processing,omitempty"`
}

type uploadReelOutput struct {
	Status      string `json:"status"`
	MediaID     string `json:"media_id,omitempty"`
	ContainerID string `json:"container_id"`
	AssetID     int64  `json:"asset_id,omitempty"`
	Message     string `json:"message"`
}

func registerUploadReel(server *mcp.Server, client instagram.Client, host hosting.VideoHost) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "upload_reel",
		Description: "Upload a video as an Instagram Reel. Provide either video_url (public URL) " +
			"or video_path (local file, requires GitHub hosting config). Creates a container, " +
			"waits for processing, then publishes. Returns the published media ID.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[uploadReelInput]) (*mcp.CallToolResultFor[uploadReelOutput], error) {
		input := params.Arguments

		videoURL := input.VideoURL
		var assetID int64

		// If a local path is provided, host it first.
		if input.VideoPath != "" && videoURL == "" {
			if host == nil {
				return errorResult[uploadReelOutput](fmt.Errorf("video_path requires GitHub hosting (set GITHUB_TOKEN and GITHUB_REPO)")), nil
			}
			var err error
			videoURL, assetID, err = host.Upload(ctx, input.VideoPath)
			if err != nil {
				return errorResult[uploadReelOutput](fmt.Errorf("hosting video: %w", err)), nil
			}
		}

		if videoURL == "" {
			return errorResult[uploadReelOutput](fmt.Errorf("either video_url or video_path is required")), nil
		}

		containerID, err := client.CreateReelContainer(ctx, instagram.ReelParams{
			VideoURL:      videoURL,
			Caption:       input.Caption,
			ShareToFeed:   input.ShareToFeed,
			ThumbOffset:   input.ThumbOffset,
			LocationID:    input.LocationID,
			CoverURL:      input.CoverURL,
			UserTags:      input.UserTags,
			Collaborators: input.Collaborators,
			AudioName:     input.AudioName,
		})
		if err != nil {
			return errorResult[uploadReelOutput](err), nil
		}

		// Async mode: return container ID for manual polling.
		if input.WaitForProcessing != nil && !*input.WaitForProcessing {
			return okResult(uploadReelOutput{
				Status:      "container_created",
				ContainerID: containerID,
				AssetID:     assetID,
				Message:     "Container created. Use check_container_status to poll, then publish_media.",
			}), nil
		}

		// Sync mode: wait → publish.
		if _, err := client.WaitForContainer(ctx, containerID); err != nil {
			return errorResult[uploadReelOutput](err), nil
		}

		pub, err := client.Publish(ctx, containerID)
		if err != nil {
			return errorResult[uploadReelOutput](err), nil
		}

		return okResult(uploadReelOutput{
			Status:      "published",
			MediaID:     pub.ID,
			ContainerID: containerID,
			AssetID:     assetID,
			Message:     "Reel uploaded and published successfully!",
		}), nil
	})
}

// ---------------------------------------------------------------------------
// upload_image_post
// ---------------------------------------------------------------------------

type uploadImageInput struct {
	ImageURL   string              `json:"image_url"`
	Caption    string              `json:"caption,omitempty"`
	LocationID string              `json:"location_id,omitempty"`
	UserTags   []instagram.UserTag `json:"user_tags,omitempty"`
	AltText    string              `json:"alt_text,omitempty"`
}

type uploadImageOutput struct {
	Status      string `json:"status"`
	MediaID     string `json:"media_id"`
	ContainerID string `json:"container_id"`
	Message     string `json:"message"`
}

func registerUploadImage(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upload_image_post",
		Description: "Upload a single JPEG image as an Instagram Feed post.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[uploadImageInput]) (*mcp.CallToolResultFor[uploadImageOutput], error) {
		input := params.Arguments
		containerID, err := client.CreateImageContainer(ctx, instagram.ImageParams{
			ImageURL:   input.ImageURL,
			Caption:    input.Caption,
			LocationID: input.LocationID,
			UserTags:   input.UserTags,
			AltText:    input.AltText,
		})
		if err != nil {
			return errorResult[uploadImageOutput](err), nil
		}

		pub, err := client.Publish(ctx, containerID)
		if err != nil {
			return errorResult[uploadImageOutput](err), nil
		}

		return okResult(uploadImageOutput{
			Status:      "published",
			MediaID:     pub.ID,
			ContainerID: containerID,
			Message:     "Image post published successfully!",
		}), nil
	})
}

// ---------------------------------------------------------------------------
// upload_carousel
// ---------------------------------------------------------------------------

type uploadCarouselInput struct {
	Children      []string `json:"children"`
	Caption       string   `json:"caption,omitempty"`
	LocationID    string   `json:"location_id,omitempty"`
	Collaborators []string `json:"collaborators,omitempty"`
}

type uploadCarouselOutput struct {
	Status              string `json:"status"`
	MediaID             string `json:"media_id"`
	CarouselContainerID string `json:"carousel_container_id"`
	Message             string `json:"message"`
}

func registerUploadCarousel(server *mcp.Server, client instagram.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "upload_carousel",
		Description: "Publish a carousel post with 2–10 images/videos. " +
			"Pass container IDs created via upload_image_post or upload_reel.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[uploadCarouselInput]) (*mcp.CallToolResultFor[uploadCarouselOutput], error) {
		input := params.Arguments
		if len(input.Children) < 2 || len(input.Children) > 10 {
			return errorResult[uploadCarouselOutput](fmt.Errorf("carousel requires 2–10 children, got %d", len(input.Children))), nil
		}

		carouselID, err := client.CreateCarouselContainer(ctx, instagram.CarouselParams{
			Children:      input.Children,
			Caption:       input.Caption,
			LocationID:    input.LocationID,
			Collaborators: input.Collaborators,
		})
		if err != nil {
			return errorResult[uploadCarouselOutput](err), nil
		}

		pub, err := client.Publish(ctx, carouselID)
		if err != nil {
			return errorResult[uploadCarouselOutput](err), nil
		}

		return okResult(uploadCarouselOutput{
			Status:              "published",
			MediaID:             pub.ID,
			CarouselContainerID: carouselID,
			Message:             "Carousel post published successfully!",
		}), nil
	})
}
