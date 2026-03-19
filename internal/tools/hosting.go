package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vikrant/instagram-mcp/internal/hosting"
)

// ---------------------------------------------------------------------------
// host_video
// ---------------------------------------------------------------------------

type hostVideoInput struct {
	VideoPath string `json:"video_path"`
}

type hostVideoOutput struct {
	Status  string `json:"status"`
	URL     string `json:"url"`
	AssetID int64  `json:"asset_id"`
	Message string `json:"message"`
}

func registerHostVideo(server *mcp.Server, host hosting.VideoHost) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "host_video",
		Description: "Upload a local video file to GitHub Releases so it has a public URL. " +
			"Use the returned URL with upload_reel. Use delete_hosted_asset to clean up after publishing.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[hostVideoInput]) (*mcp.CallToolResultFor[hostVideoOutput], error) {
		input := params.Arguments
		url, assetID, err := host.Upload(ctx, input.VideoPath)
		if err != nil {
			return errorResult[hostVideoOutput](err), nil
		}

		return okResult(hostVideoOutput{
			Status:  "hosted",
			URL:     url,
			AssetID: assetID,
			Message: "Video uploaded to GitHub Releases. Use this URL with upload_reel.",
		}), nil
	})
}

// ---------------------------------------------------------------------------
// delete_hosted_asset
// ---------------------------------------------------------------------------

type deleteAssetInput struct {
	AssetID int64 `json:"asset_id"`
}

type deleteAssetOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func registerDeleteHostedAsset(server *mcp.Server, host hosting.VideoHost) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_hosted_asset",
		Description: "Delete a previously hosted video from GitHub Releases. Pass the asset_id returned by host_video or upload_reel.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[deleteAssetInput]) (*mcp.CallToolResultFor[deleteAssetOutput], error) {
		if err := host.Delete(ctx, params.Arguments.AssetID); err != nil {
			return errorResult[deleteAssetOutput](err), nil
		}

		return okResult(deleteAssetOutput{
			Status:  "deleted",
			Message: "Hosted asset deleted successfully.",
		}), nil
	})
}
