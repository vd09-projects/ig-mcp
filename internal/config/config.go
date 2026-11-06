// Package config loads and validates configuration for the Instagram MCP
// server.
//
// Secrets (tokens, account IDs) come only from the environment / env file.
// Tunables (timeouts, limits, concurrency) come from an optional YAML config
// file. Precedence, lowest to highest: built-in defaults < config file < env.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration required by the Instagram MCP server.
// Fields are validated at load time; the program must not start if
// validation fails.
type Config struct {
	// AccessToken is a long-lived Facebook Graph API token with
	// instagram_basic, instagram_content_publish, and pages_read_engagement.
	AccessToken string `yaml:"-"`

	// AccountID is the numeric Instagram Business/Creator account ID.
	AccountID string `yaml:"-"`

	// GitHubToken is a personal access token with repo scope for uploading
	// release assets. Optional — if empty, local-file hosting is disabled.
	GitHubToken string `yaml:"-"`

	// GitHubRepo is the target repository in "owner/repo" format.
	// Optional — required only when GitHubToken is set.
	GitHubRepo string `yaml:"-"`

	GraphAPI GraphAPIConfig `yaml:"graph_api"`
	Polling  PollingConfig  `yaml:"polling"`
	Insights InsightsConfig `yaml:"insights"`
	Tools    ToolsConfig    `yaml:"tools"`
	Hosting  HostingConfig  `yaml:"hosting"`
}

// GraphAPIConfig tunes the Graph API transport.
type GraphAPIConfig struct {
	// Version is the Facebook Graph API version (e.g. "v21.0").
	Version string `yaml:"version"`
	// Timeout bounds a single Graph API HTTP request.
	Timeout time.Duration `yaml:"timeout"`
}

// PollingConfig tunes container-status polling while Instagram processes video.
type PollingConfig struct {
	// Interval is how long to wait between container-status checks.
	Interval time.Duration `yaml:"interval"`
	// MaxAttempts is the maximum number of status polls before we declare a timeout.
	MaxAttempts int `yaml:"max_attempts"`
}

// InsightsConfig tunes insight fetching.
type InsightsConfig struct {
	// Concurrency bounds parallel insight requests to stay under Graph API rate limits.
	Concurrency int `yaml:"concurrency"`
}

// ToolsConfig holds per-tool defaults and caps.
type ToolsConfig struct {
	// MaxMediaLimit caps how many media items a single tool call will page through.
	MaxMediaLimit int `yaml:"max_media_limit"`
	// ListMediaDefault is list_recent_media's limit when the caller omits one.
	ListMediaDefault int `yaml:"list_media_default"`
	// ReelsDefault is get_reels_analytics's limit when the caller omits one.
	ReelsDefault int `yaml:"reels_default"`
	// CleanupOlderThan is cleanup_hosted_assets's age threshold when the caller
	// omits one. A normal upload→publish→delete cycle takes well under an hour.
	CleanupOlderThan time.Duration `yaml:"cleanup_older_than"`
}

// HostingConfig tunes GitHub Release video hosting.
type HostingConfig struct {
	// UploadTimeout bounds GitHub API calls, including release-asset uploads.
	// Uploads to uploads.github.com can run ~1.3 Mbps: a 100 MB reel takes
	// ~10 min, so the default leaves headroom.
	UploadTimeout time.Duration `yaml:"upload_timeout"`
	// RequestTimeout bounds the HEAD request that resolves the CDN URL.
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// ReleaseTag is the GitHub Release that holds hosted assets.
	ReleaseTag string `yaml:"release_tag"`
}

// Default returns the built-in configuration, before any file or env overrides.
func Default() *Config {
	return &Config{
		GraphAPI: GraphAPIConfig{Version: "v21.0", Timeout: 30 * time.Second},
		Polling:  PollingConfig{Interval: 5 * time.Second, MaxAttempts: 60},
		Insights: InsightsConfig{Concurrency: 5},
		Tools: ToolsConfig{
			MaxMediaLimit:    500,
			ListMediaDefault: 10,
			ReelsDefault:     25,
			CleanupOlderThan: time.Hour,
		},
		Hosting: HostingConfig{
			UploadTimeout:  30 * time.Minute,
			RequestTimeout: 30 * time.Second,
			ReleaseTag:     "media-assets",
		},
	}
}

// Load reads configuration from an env file, an optional YAML config file and
// environment variables, applies defaults, and validates the result.
//
// envFile precedence:
//  1. The envFile argument (non-empty string passed by the caller)
//  2. The ENV_FILE environment variable
//  3. ".env" in the current working directory (best-effort, ignored if missing)
//
// configFile precedence:
//  1. The configFile argument
//  2. The CONFIG_FILE environment variable (may itself be set in the env file)
//  3. None — built-in defaults are used
func Load(envFile, configFile string) (*Config, error) {
	if envFile == "" {
		envFile = os.Getenv("ENV_FILE")
	}

	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return nil, fmt.Errorf("loading env file %q: %w", envFile, err)
		}
	} else {
		// Best-effort: load .env without failing if the file is missing.
		_ = godotenv.Load()
	}

	cfg := Default()

	if configFile == "" {
		configFile = os.Getenv("CONFIG_FILE")
	}
	if configFile != "" {
		if err := cfg.loadFile(configFile); err != nil {
			return nil, err
		}
	}

	cfg.AccessToken = os.Getenv("INSTAGRAM_ACCESS_TOKEN")
	cfg.AccountID = os.Getenv("INSTAGRAM_ACCOUNT_ID")
	cfg.GitHubToken = os.Getenv("GITHUB_TOKEN")
	cfg.GitHubRepo = os.Getenv("GITHUB_REPO")

	if err := cfg.applyEnvOverrides(); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadFile overlays the YAML file at path onto c. Keys absent from the file
// keep their current values; unknown keys are rejected to catch typos.
func (c *Config) loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening config file %q: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("parsing config file %q: %w", path, err)
	}
	return nil
}

// applyEnvOverrides lets the legacy env vars override file/default values.
func (c *Config) applyEnvOverrides() error {
	if v := os.Getenv("GRAPH_API_VERSION"); v != "" {
		c.GraphAPI.Version = v
	}

	if v := os.Getenv("STATUS_POLL_INTERVAL_MS"); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("STATUS_POLL_INTERVAL_MS: expected integer, got %q", v)
		}
		c.Polling.Interval = time.Duration(ms) * time.Millisecond
	}

	if v := os.Getenv("STATUS_POLL_MAX_ATTEMPTS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("STATUS_POLL_MAX_ATTEMPTS: expected integer, got %q", v)
		}
		c.Polling.MaxAttempts = n
	}

	return nil
}

// BaseURL returns the versioned Graph API base URL.
func (c *Config) BaseURL() string {
	return "https://graph.instagram.com/" + c.GraphAPI.Version
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
	if c.GitHubToken != "" && c.GitHubRepo == "" {
		errs = append(errs, errors.New("GITHUB_REPO is required when GITHUB_TOKEN is set"))
	}

	if c.GraphAPI.Version == "" {
		errs = append(errs, errors.New("graph_api.version must not be empty"))
	}
	if c.Hosting.ReleaseTag == "" {
		errs = append(errs, errors.New("hosting.release_tag must not be empty"))
	}

	for _, d := range []struct {
		name string
		v    time.Duration
	}{
		{"graph_api.timeout", c.GraphAPI.Timeout},
		{"polling.interval", c.Polling.Interval},
		{"tools.cleanup_older_than", c.Tools.CleanupOlderThan},
		{"hosting.upload_timeout", c.Hosting.UploadTimeout},
		{"hosting.request_timeout", c.Hosting.RequestTimeout},
	} {
		if d.v <= 0 {
			errs = append(errs, fmt.Errorf("%s must be positive, got %s", d.name, d.v))
		}
	}

	for _, n := range []struct {
		name string
		v    int
	}{
		{"polling.max_attempts", c.Polling.MaxAttempts},
		{"insights.concurrency", c.Insights.Concurrency},
		{"tools.max_media_limit", c.Tools.MaxMediaLimit},
	} {
		if n.v <= 0 {
			errs = append(errs, fmt.Errorf("%s must be positive, got %d", n.name, n.v))
		}
	}

	for _, n := range []struct {
		name string
		v    int
	}{
		{"tools.list_media_default", c.Tools.ListMediaDefault},
		{"tools.reels_default", c.Tools.ReelsDefault},
	} {
		if n.v <= 0 || n.v > c.Tools.MaxMediaLimit {
			errs = append(errs, fmt.Errorf("%s must be between 1 and tools.max_media_limit (%d), got %d",
				n.name, c.Tools.MaxMediaLimit, n.v))
		}
	}

	return errors.Join(errs...)
}
