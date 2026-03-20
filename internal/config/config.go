// Package config loads and validates environment-based configuration
// for the Instagram MCP server.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration required by the Instagram MCP server.
// Fields are validated at load time; the program must not start if
// validation fails.
type Config struct {
	// AccessToken is a long-lived Facebook Graph API token with
	// instagram_basic, instagram_content_publish, and pages_read_engagement.
	AccessToken string

	// AccountID is the numeric Instagram Business/Creator account ID.
	AccountID string

	// GraphAPIVersion is the Facebook Graph API version (e.g. "v21.0").
	GraphAPIVersion string

	// PollInterval is how long to wait between container-status checks
	// while a video is being processed by Instagram.
	PollInterval time.Duration

	// PollMaxAttempts is the maximum number of status polls before we
	// declare a timeout.
	PollMaxAttempts int

	// GitHubToken is a personal access token with repo scope for uploading
	// release assets. Optional — if empty, local-file hosting is disabled.
	GitHubToken string

	// GitHubRepo is the target repository in "owner/repo" format.
	// Optional — required only when GitHubToken is set.
	GitHubRepo string
}

// Load reads configuration from a .env file (if present) and environment
// variables, applies defaults, and validates required fields.
func Load() (*Config, error) {
	// Best-effort: load .env without failing if the file is missing.
	_ = godotenv.Load()

	cfg := &Config{
		AccessToken:     os.Getenv("INSTAGRAM_ACCESS_TOKEN"),
		AccountID:       os.Getenv("INSTAGRAM_ACCOUNT_ID"),
		GraphAPIVersion: envOrDefault("GRAPH_API_VERSION", "v21.0"),
		GitHubToken:     os.Getenv("GITHUB_TOKEN"),
		GitHubRepo:      os.Getenv("GITHUB_REPO"),
	}

	pollMS, err := envIntOrDefault("STATUS_POLL_INTERVAL_MS", 5000)
	if err != nil {
		return nil, fmt.Errorf("STATUS_POLL_INTERVAL_MS: %w", err)
	}
	cfg.PollInterval = time.Duration(pollMS) * time.Millisecond

	cfg.PollMaxAttempts, err = envIntOrDefault("STATUS_POLL_MAX_ATTEMPTS", 60)
	if err != nil {
		return nil, fmt.Errorf("STATUS_POLL_MAX_ATTEMPTS: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// BaseURL returns the versioned Graph API base URL.
func (c *Config) BaseURL() string {
	return "https://graph.instagram.com/" + c.GraphAPIVersion
}

// GitHubHostingEnabled reports whether GitHub Release hosting is configured.
func (c *Config) GitHubHostingEnabled() bool {
	return c.GitHubToken != "" && c.GitHubRepo != ""
}

func (c *Config) validate() error {
	var errs []error
	if c.AccessToken == "" {
		errs = append(errs, errors.New("INSTAGRAM_ACCESS_TOKEN is required"))
	}
	if c.AccountID == "" {
		errs = append(errs, errors.New("INSTAGRAM_ACCOUNT_ID is required"))
	}
	if c.PollInterval <= 0 {
		errs = append(errs, errors.New("STATUS_POLL_INTERVAL_MS must be positive"))
	}
	if c.PollMaxAttempts <= 0 {
		errs = append(errs, errors.New("STATUS_POLL_MAX_ATTEMPTS must be positive"))
	}
	if c.GitHubToken != "" && c.GitHubRepo == "" {
		errs = append(errs, errors.New("GITHUB_REPO is required when GITHUB_TOKEN is set"))
	}
	return errors.Join(errs...)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", v)
	}
	return n, nil
}
