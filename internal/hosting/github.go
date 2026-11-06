// Package hosting provides video hosting backends that make local files
// publicly accessible so the Instagram Graph API can fetch them.
package hosting

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
)

// VideoHost uploads local video files to a public host and returns
// a URL that the Instagram Graph API can fetch.
type VideoHost interface {
	// Upload makes a local file publicly accessible and returns its URL
	// along with an identifier that can be used to delete it later.
	Upload(ctx context.Context, filePath string) (publicURL string, assetID int64, err error)

	// Delete removes a previously uploaded asset.
	Delete(ctx context.Context, assetID int64) error

	// Cleanup bulk-removes hosted assets older than olderThan. When dryRun is
	// true it reports what would be deleted without deleting anything. It is
	// safe to call repeatedly (idempotent) and needs no caller-supplied IDs —
	// the host discovers its own assets.
	Cleanup(ctx context.Context, olderThan time.Duration, dryRun bool) (CleanupResult, error)
}

// AssetInfo describes a single hosted asset for cleanup reporting.
type AssetInfo struct {
	AssetID   int64  `json:"asset_id"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	AgeSecs   int64  `json:"age_secs"`
}

// CleanupResult reports the outcome of a Cleanup call.
type CleanupResult struct {
	DryRun     bool        `json:"dry_run"`
	Deleted    []AssetInfo `json:"deleted"`
	Kept       []AssetInfo `json:"kept"`
	FreedBytes int64       `json:"freed_bytes"`
}

// GitHubOptions tunes a GitHubHost.
type GitHubOptions struct {
	// UploadTimeout bounds GitHub API calls, including release-asset uploads.
	UploadTimeout time.Duration
	// RequestTimeout bounds the HEAD request that resolves the CDN URL.
	RequestTimeout time.Duration
	// ReleaseTag is the GitHub Release that holds hosted assets.
	ReleaseTag string
}

// GitHubHost hosts videos as GitHub Release assets.
type GitHubHost struct {
	client         *github.Client
	owner          string
	repo           string
	releaseTag     string
	requestTimeout time.Duration
}

// NewGitHubHost creates a VideoHost backed by GitHub Releases.
// repoFullName must be in "owner/repo" format.
func NewGitHubHost(token, repoFullName string, opts GitHubOptions) (*GitHubHost, error) {
	owner, repo, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || repo == "" {
		return nil, fmt.Errorf("GITHUB_REPO must be in owner/repo format, got %q", repoFullName)
	}

	httpClient := &http.Client{Timeout: opts.UploadTimeout}
	client := github.NewClient(httpClient).WithAuthToken(token)

	return &GitHubHost{
		client:         client,
		owner:          owner,
		repo:           repo,
		releaseTag:     opts.ReleaseTag,
		requestTimeout: opts.RequestTimeout,
	}, nil
}

// Upload uploads a local file to the media-assets GitHub Release and returns
// its public BrowserDownloadURL.
func (h *GitHubHost) Upload(ctx context.Context, filePath string) (string, int64, error) {
	log.Printf("[github-host] opening file %q", filePath)
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	fi, _ := f.Stat()
	if fi != nil {
		log.Printf("[github-host] file size: %.1f MB", float64(fi.Size())/(1024*1024))
	}

	release, err := h.getOrCreateRelease(ctx)
	if err != nil {
		return "", 0, err
	}
	log.Printf("[github-host] using release id=%d tag=%s", release.GetID(), h.releaseTag)

	// Unique asset name with .mp4 extension to avoid collisions
	// and ensure correct Content-Type on download.
	name := fmt.Sprintf("%d.mp4", time.Now().UnixMilli())

	log.Printf("[github-host] uploading asset %q to %s/%s …", name, h.owner, h.repo)
	asset, _, err := h.client.Repositories.UploadReleaseAsset(
		ctx, h.owner, h.repo, release.GetID(),
		&github.UploadOptions{
			Name:      name,
			MediaType: "video/mp4",
		},
		f,
	)
	if err != nil {
		return "", 0, fmt.Errorf("uploading release asset: %w", err)
	}
	log.Printf("[github-host] asset uploaded — asset_id=%d", asset.GetID())

	// The github.com download URL is blocked by GitHub's robots.txt, causing
	// Instagram's servers to get a 403. Instead, resolve the redirect chain to
	// get the actual CDN URL (objects.githubusercontent.com) which is accessible.
	browserURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		h.owner, h.repo, h.releaseTag, name,
	)

	cdnURL, err := resolveRedirect(ctx, browserURL, h.requestTimeout)
	if err != nil {
		log.Printf("[github-host] redirect resolution failed, using browser URL: %v", err)
		return browserURL, asset.GetID(), nil
	}
	log.Printf("[github-host] resolved CDN URL: %s", cdnURL)

	return cdnURL, asset.GetID(), nil
}

// Delete removes a release asset by ID.
func (h *GitHubHost) Delete(ctx context.Context, assetID int64) error {
	_, err := h.client.Repositories.DeleteReleaseAsset(ctx, h.owner, h.repo, assetID)
	if err != nil {
		return fmt.Errorf("deleting release asset: %w", err)
	}
	return nil
}

// Cleanup lists every asset on the media-assets release and deletes those
// older than olderThan. The host owns the release, so no asset IDs are
// required from the caller — it paginates, filters by age, and deletes
// server-side. A missing release is treated as "nothing to clean".
func (h *GitHubHost) Cleanup(ctx context.Context, olderThan time.Duration, dryRun bool) (CleanupResult, error) {
	result := CleanupResult{DryRun: dryRun, Deleted: []AssetInfo{}, Kept: []AssetInfo{}}

	rel, resp, err := h.client.Repositories.GetReleaseByTag(ctx, h.owner, h.repo, h.releaseTag)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// No release yet — nothing has ever been hosted.
			return result, nil
		}
		return result, fmt.Errorf("looking up release %q: %w", h.releaseTag, err)
	}

	now := time.Now()
	opt := &github.ListOptions{PerPage: 100}
	for {
		assets, listResp, err := h.client.Repositories.ListReleaseAssets(ctx, h.owner, h.repo, rel.GetID(), opt)
		if err != nil {
			return result, fmt.Errorf("listing release assets: %w", err)
		}

		for _, a := range assets {
			age := now.Sub(a.GetCreatedAt().Time)
			info := AssetInfo{
				AssetID:   a.GetID(),
				Name:      a.GetName(),
				SizeBytes: int64(a.GetSize()),
				AgeSecs:   int64(age.Seconds()),
			}

			if age < olderThan {
				result.Kept = append(result.Kept, info)
				continue
			}

			if !dryRun {
				if _, err := h.client.Repositories.DeleteReleaseAsset(ctx, h.owner, h.repo, a.GetID()); err != nil {
					return result, fmt.Errorf("deleting asset %d (%s): %w", a.GetID(), a.GetName(), err)
				}
			}
			result.Deleted = append(result.Deleted, info)
			result.FreedBytes += info.SizeBytes
		}

		if listResp.NextPage == 0 {
			break
		}
		opt.Page = listResp.NextPage
	}

	log.Printf("[github-host] cleanup: deleted=%d kept=%d freed=%.1f MB dryRun=%v",
		len(result.Deleted), len(result.Kept), float64(result.FreedBytes)/(1024*1024), dryRun)

	return result, nil
}

// resolveRedirect follows the redirect chain for a URL and returns the final
// destination URL without downloading the body. This is used to resolve GitHub
// release download URLs to their CDN URLs (objects.githubusercontent.com),
// which are not blocked by GitHub's robots.txt.
func resolveRedirect(ctx context.Context, rawURL string, timeout time.Duration) (string, error) {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Stop following redirects — we just want the final Location.
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolving redirect: %w", err)
	}
	resp.Body.Close()

	finalURL := resp.Request.URL.String()
	if finalURL == rawURL {
		return "", fmt.Errorf("no redirect occurred")
	}
	return finalURL, nil
}

// getOrCreateRelease returns the media-assets release, creating it if needed.
func (h *GitHubHost) getOrCreateRelease(ctx context.Context) (*github.RepositoryRelease, error) {
	rel, resp, err := h.client.Repositories.GetReleaseByTag(ctx, h.owner, h.repo, h.releaseTag)
	if err == nil {
		return rel, nil
	}
	if resp == nil || resp.StatusCode != 404 {
		return nil, fmt.Errorf("looking up release %q: %w", h.releaseTag, err)
	}

	// Create the release.
	rel, _, err = h.client.Repositories.CreateRelease(ctx, h.owner, h.repo, &github.RepositoryRelease{
		TagName: github.Ptr(h.releaseTag),
		Name:    github.Ptr("Media Assets"),
		Body:    github.Ptr("Ephemeral video hosting for Instagram uploads. Assets are deleted after publishing."),
	})
	if err != nil {
		return nil, fmt.Errorf("creating release %q: %w", h.releaseTag, err)
	}
	return rel, nil
}
