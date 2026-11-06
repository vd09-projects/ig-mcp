package tools

import (
	"context"
	"fmt"
	"time"

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

// ---------------------------------------------------------------------------
// cleanup_hosted_assets
// ---------------------------------------------------------------------------

type cleanupAssetsInput struct {
	OlderThanSecs int64 `json:"older_than_secs,omitempty"`
	DryRun        bool  `json:"dry_run,omitempty"`
}

type cleanupAssetsOutput struct {
	Status       string              `json:"status"`
	DryRun       bool                `json:"dry_run"`
	DeletedCount int                 `json:"deleted_count"`
	KeptCount    int                 `json:"kept_count"`
	FreedBytes   int64               `json:"freed_bytes"`
	FreedMB      float64             `json:"freed_mb"`
	Deleted      []hosting.AssetInfo `json:"deleted"`
	Kept         []hosting.AssetInfo `json:"kept"`
	Message      string              `json:"message"`
}

func registerCleanupHostedAssets(server *mcp.Server, host hosting.VideoHost, opts Options) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "cleanup_hosted_assets",
		Description: "Bulk-delete orphaned videos from the GitHub Releases CDN used for temporary Instagram hosting. " +
			fmt.Sprintf("Deletes every hosted asset older than older_than_secs (default %ds); a normal upload→publish→delete ",
				int64(opts.CleanupOlderThan.Seconds())) +
			"cycle finishes well under that, so anything older is leftover junk. Set dry_run=true to preview without deleting. " +
			"Needs no asset IDs — the server discovers and removes its own assets. Safe to call repeatedly.",
	}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[cleanupAssetsInput]) (*mcp.CallToolResultFor[cleanupAssetsOutput], error) {
		olderThanSecs := params.Arguments.OlderThanSecs
		if olderThanSecs <= 0 {
			olderThanSecs = int64(opts.CleanupOlderThan.Seconds())
		}

		res, err := host.Cleanup(ctx, time.Duration(olderThanSecs)*time.Second, params.Arguments.DryRun)
		if err != nil {
			return errorResult[cleanupAssetsOutput](err), nil
		}

		status := "cleaned"
		verb := "Deleted"
		if res.DryRun {
			status = "dry_run"
			verb = "Would delete"
		}
		freedMB := float64(res.FreedBytes) / (1024 * 1024)

		return okResult(cleanupAssetsOutput{
			Status:       status,
			DryRun:       res.DryRun,
			DeletedCount: len(res.Deleted),
			KeptCount:    len(res.Kept),
			FreedBytes:   res.FreedBytes,
			FreedMB:      freedMB,
			Deleted:      res.Deleted,
			Kept:         res.Kept,
			Message: fmt.Sprintf("%s %d asset(s) (%.1f MB), kept %d newer than %ds.",
				verb, len(res.Deleted), freedMB, len(res.Kept), olderThanSecs),
		}), nil
	})
}
