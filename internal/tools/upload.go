package tools

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/hosting"
	"github.com/vikrant/instagram-mcp/internal/instagram"
)

// logInfo sends an info-level log notification to the MCP client so callers
// can see progress in real time, and also writes to stderr for server-side
// debugging.
func logInfo(ctx context.Context, ss *mcp.ServerSession, logger, msg string) {
	log.Printf("[%s] %s", logger, msg)
	if ss == nil {
		return
	}
	_ = ss.Log(ctx, &mcp.LoggingMessageParams{
		Level:  "info",
		Logger: logger,
		Data:   msg,
	})
}

// logError is like logInfo but at error level.
func logError(ctx context.Context, ss *mcp.ServerSession, logger, msg string) {
	log.Printf("[%s] ERROR: %s", logger, msg)
	if ss == nil {
		return
	}
	_ = ss.Log(ctx, &mcp.LoggingMessageParams{
		Level:  "error",
		Logger: logger,
		Data:   msg,
	})
}

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
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[uploadReelInput]) (*mcp.CallToolResultFor[uploadReelOutput], error) {
		const logger = "upload_reel"
		input := params.Arguments
		logInfo(ctx, ss, logger, fmt.Sprintf("starting — video_path=%q video_url=%q caption=%q", input.VideoPath, input.VideoURL, input.Caption))

		videoURL := input.VideoURL
		var assetID int64

		// If a local path is provided, host it first.
		if input.VideoPath != "" && videoURL == "" {
			if host == nil {
				logError(ctx, ss, logger, "no hosting configured — set GITHUB_TOKEN and GITHUB_REPO")
				return errorResult[uploadReelOutput](fmt.Errorf("video_path requires GitHub hosting (set GITHUB_TOKEN and GITHUB_REPO)")), nil
			}
			logInfo(ctx, ss, logger, "hosting local file via GitHub…")
			var err error
			videoURL, assetID, err = host.Upload(ctx, input.VideoPath)
			if err != nil {
				logError(ctx, ss, logger, fmt.Sprintf("hosting video: %v", err))
				return errorResult[uploadReelOutput](fmt.Errorf("hosting video: %w", err)), nil
			}
			logInfo(ctx, ss, logger, fmt.Sprintf("hosted — url=%s asset_id=%d", videoURL, assetID))
		}

		if videoURL == "" {
			logError(ctx, ss, logger, "no video_url or video_path provided")
			return errorResult[uploadReelOutput](fmt.Errorf("either video_url or video_path is required")), nil
		}

		logInfo(ctx, ss, logger, "creating reel container…")
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
			logError(ctx, ss, logger, fmt.Sprintf("creating container: %v", err))
			return errorResult[uploadReelOutput](err), nil
		}
		logInfo(ctx, ss, logger, fmt.Sprintf("container created — id=%s", containerID))

		// Async mode: return container ID for manual polling.
		if input.WaitForProcessing != nil && !*input.WaitForProcessing {
			logInfo(ctx, ss, logger, fmt.Sprintf("async mode — returning container_id=%s", containerID))
			return okResult(uploadReelOutput{
				Status:      "container_created",
				ContainerID: containerID,
				AssetID:     assetID,
				Message:     "Container created. Use check_container_status to poll, then publish_media.",
			}), nil
		}

		// Sync mode: wait → publish.
		logInfo(ctx, ss, logger, "waiting for container processing…")
		if _, err := client.WaitForContainer(ctx, containerID); err != nil {
			logError(ctx, ss, logger, fmt.Sprintf("waiting for container: %v", err))
			return errorResult[uploadReelOutput](err), nil
		}
		logInfo(ctx, ss, logger, "container finished processing")

		logInfo(ctx, ss, logger, "publishing…")
		pub, err := client.Publish(ctx, containerID)
		if err != nil {
			logError(ctx, ss, logger, fmt.Sprintf("publishing: %v", err))
			return errorResult[uploadReelOutput](err), nil
		}
		logInfo(ctx, ss, logger, fmt.Sprintf("published — media_id=%s", pub.ID))

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
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[uploadImageInput]) (*mcp.CallToolResultFor[uploadImageOutput], error) {
		const logger = "upload_image"
		input := params.Arguments
		logInfo(ctx, ss, logger, fmt.Sprintf("starting — image_url=%q caption=%q", input.ImageURL, input.Caption))

		containerID, err := client.CreateImageContainer(ctx, instagram.ImageParams{
			ImageURL:   input.ImageURL,
			Caption:    input.Caption,
			LocationID: input.LocationID,
			UserTags:   input.UserTags,
			AltText:    input.AltText,
		})
		if err != nil {
			logError(ctx, ss, logger, fmt.Sprintf("creating container: %v", err))
			return errorResult[uploadImageOutput](err), nil
		}
		logInfo(ctx, ss, logger, fmt.Sprintf("container created — id=%s", containerID))

		logInfo(ctx, ss, logger, "publishing…")
		pub, err := client.Publish(ctx, containerID)
		if err != nil {
			logError(ctx, ss, logger, fmt.Sprintf("publishing: %v", err))
			return errorResult[uploadImageOutput](err), nil
		}
		logInfo(ctx, ss, logger, fmt.Sprintf("published — media_id=%s", pub.ID))

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
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[uploadCarouselInput]) (*mcp.CallToolResultFor[uploadCarouselOutput], error) {
		const logger = "upload_carousel"
		input := params.Arguments
		logInfo(ctx, ss, logger, fmt.Sprintf("starting — %d children, caption=%q", len(input.Children), input.Caption))

		if len(input.Children) < 2 || len(input.Children) > 10 {
			logError(ctx, ss, logger, fmt.Sprintf("invalid child count: %d", len(input.Children)))
			return errorResult[uploadCarouselOutput](fmt.Errorf("carousel requires 2–10 children, got %d", len(input.Children))), nil
		}

		logInfo(ctx, ss, logger, "creating carousel container…")
		carouselID, err := client.CreateCarouselContainer(ctx, instagram.CarouselParams{
			Children:      input.Children,
			Caption:       input.Caption,
			LocationID:    input.LocationID,
			Collaborators: input.Collaborators,
		})
		if err != nil {
			logError(ctx, ss, logger, fmt.Sprintf("creating container: %v", err))
			return errorResult[uploadCarouselOutput](err), nil
		}
		logInfo(ctx, ss, logger, fmt.Sprintf("container created — id=%s", carouselID))

		logInfo(ctx, ss, logger, "publishing…")
		pub, err := client.Publish(ctx, carouselID)
		if err != nil {
			logError(ctx, ss, logger, fmt.Sprintf("publishing: %v", err))
			return errorResult[uploadCarouselOutput](err), nil
		}
		logInfo(ctx, ss, logger, fmt.Sprintf("published — media_id=%s", pub.ID))

		return okResult(uploadCarouselOutput{
			Status:              "published",
			MediaID:             pub.ID,
			CarouselContainerID: carouselID,
			Message:             "Carousel post published successfully!",
		}), nil
	})
}
