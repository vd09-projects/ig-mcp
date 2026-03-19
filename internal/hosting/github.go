// Package hosting provides video hosting backends that make local files
// publicly accessible so the Instagram Graph API can fetch them.
package hosting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
}

const releaseTag = "media-assets"

// GitHubHost hosts videos as GitHub Release assets.
type GitHubHost struct {
	client *github.Client
	owner  string
	repo   string
}

// NewGitHubHost creates a VideoHost backed by GitHub Releases.
// repoFullName must be in "owner/repo" format.
func NewGitHubHost(token, repoFullName string) (*GitHubHost, error) {
	owner, repo, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || repo == "" {
		return nil, fmt.Errorf("GITHUB_REPO must be in owner/repo format, got %q", repoFullName)
	}

	client := github.NewClient(nil).WithAuthToken(token)

	return &GitHubHost{
		client: client,
		owner:  owner,
		repo:   repo,
	}, nil
}

// Upload uploads a local file to the media-assets GitHub Release and returns
// its public BrowserDownloadURL.
func (h *GitHubHost) Upload(ctx context.Context, filePath string) (string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	release, err := h.getOrCreateRelease(ctx)
	if err != nil {
		return "", 0, err
	}

	// Unique asset name to avoid collisions.
	name := fmt.Sprintf("%d-%s", time.Now().UnixMilli(), filepath.Base(filePath))

	asset, _, err := h.client.Repositories.UploadReleaseAsset(
		ctx, h.owner, h.repo, release.GetID(),
		&github.UploadOptions{Name: name},
		f,
	)
	if err != nil {
		return "", 0, fmt.Errorf("uploading release asset: %w", err)
	}

	return asset.GetBrowserDownloadURL(), asset.GetID(), nil
}

// Delete removes a release asset by ID.
func (h *GitHubHost) Delete(ctx context.Context, assetID int64) error {
	_, err := h.client.Repositories.DeleteReleaseAsset(ctx, h.owner, h.repo, assetID)
	if err != nil {
		return fmt.Errorf("deleting release asset: %w", err)
	}
	return nil
}

// getOrCreateRelease returns the media-assets release, creating it if needed.
func (h *GitHubHost) getOrCreateRelease(ctx context.Context) (*github.RepositoryRelease, error) {
	rel, resp, err := h.client.Repositories.GetReleaseByTag(ctx, h.owner, h.repo, releaseTag)
	if err == nil {
		return rel, nil
	}
	if resp == nil || resp.StatusCode != 404 {
		return nil, fmt.Errorf("looking up release %q: %w", releaseTag, err)
	}

	// Create the release.
	rel, _, err = h.client.Repositories.CreateRelease(ctx, h.owner, h.repo, &github.RepositoryRelease{
		TagName: github.Ptr(releaseTag),
		Name:    github.Ptr("Media Assets"),
		Body:    github.Ptr("Ephemeral video hosting for Instagram uploads. Assets are deleted after publishing."),
	})
	if err != nil {
		return nil, fmt.Errorf("creating release %q: %w", releaseTag, err)
	}
	return rel, nil
}
