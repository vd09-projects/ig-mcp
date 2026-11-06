package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vikrant/instagram-mcp/internal/config"
)

func TestLoad_MissingRequiredFields(t *testing.T) {
	os.Unsetenv("INSTAGRAM_ACCESS_TOKEN")
	os.Unsetenv("INSTAGRAM_ACCOUNT_ID")

	_, err := config.Load("", "")
	if err == nil {
		t.Fatal("expected error when required env vars are missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "test-token")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "123456")

	cfg, err := config.Load("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GraphAPI.Version != "v21.0" {
		t.Errorf("expected default API version v21.0, got %s", cfg.GraphAPI.Version)
	}
	if cfg.Polling.MaxAttempts != 60 {
		t.Errorf("expected default poll max 60, got %d", cfg.Polling.MaxAttempts)
	}
	if cfg.BaseURL() != "https://graph.instagram.com/v21.0" {
		t.Errorf("unexpected base URL: %s", cfg.BaseURL())
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	t.Setenv("GRAPH_API_VERSION", "v22.0")
	t.Setenv("STATUS_POLL_INTERVAL_MS", "2000")
	t.Setenv("STATUS_POLL_MAX_ATTEMPTS", "10")

	cfg, err := config.Load("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GraphAPI.Version != "v22.0" {
		t.Errorf("expected v22.0, got %s", cfg.GraphAPI.Version)
	}
	if cfg.Polling.MaxAttempts != 10 {
		t.Errorf("expected 10, got %d", cfg.Polling.MaxAttempts)
	}
}

func TestLoad_InvalidPollInterval(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	t.Setenv("STATUS_POLL_INTERVAL_MS", "not-a-number")

	_, err := config.Load("", "")
	if err == nil {
		t.Fatal("expected error for non-numeric poll interval")
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_ConfigFile(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	path := writeConfig(t, `
graph_api:
  timeout: 45s
insights:
  concurrency: 2
tools:
  max_media_limit: 200
  reels_default: 50
hosting:
  upload_timeout: 1h
  release_tag: my-assets
`)

	cfg, err := config.Load("", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GraphAPI.Timeout != 45*time.Second {
		t.Errorf("expected graph timeout 45s, got %s", cfg.GraphAPI.Timeout)
	}
	if cfg.Insights.Concurrency != 2 {
		t.Errorf("expected concurrency 2, got %d", cfg.Insights.Concurrency)
	}
	if cfg.Tools.MaxMediaLimit != 200 || cfg.Tools.ReelsDefault != 50 {
		t.Errorf("unexpected tools config: %+v", cfg.Tools)
	}
	if cfg.Hosting.UploadTimeout != time.Hour || cfg.Hosting.ReleaseTag != "my-assets" {
		t.Errorf("unexpected hosting config: %+v", cfg.Hosting)
	}
	// Keys absent from the file keep their defaults.
	if cfg.GraphAPI.Version != "v21.0" || cfg.Tools.ListMediaDefault != 10 || cfg.Hosting.RequestTimeout != 30*time.Second {
		t.Errorf("expected defaults for unset keys, got %+v", cfg)
	}
}

func TestLoad_ConfigFileFromEnv(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	t.Setenv("CONFIG_FILE", writeConfig(t, "insights:\n  concurrency: 3\n"))

	cfg, err := config.Load("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Insights.Concurrency != 3 {
		t.Errorf("expected concurrency 3, got %d", cfg.Insights.Concurrency)
	}
}

func TestLoad_EnvOverridesConfigFile(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	t.Setenv("GRAPH_API_VERSION", "v23.0")
	t.Setenv("STATUS_POLL_INTERVAL_MS", "1500")
	path := writeConfig(t, "graph_api:\n  version: v22.0\npolling:\n  interval: 10s\n  max_attempts: 5\n")

	cfg, err := config.Load("", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GraphAPI.Version != "v23.0" {
		t.Errorf("expected env version v23.0, got %s", cfg.GraphAPI.Version)
	}
	if cfg.Polling.Interval != 1500*time.Millisecond {
		t.Errorf("expected env interval 1.5s, got %s", cfg.Polling.Interval)
	}
	if cfg.Polling.MaxAttempts != 5 {
		t.Errorf("expected file max attempts 5, got %d", cfg.Polling.MaxAttempts)
	}
}

func TestLoad_ConfigFileErrors(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")

	for name, tc := range map[string]struct{ body, wantErr string }{
		"unknown key":         {"tools:\n  max_media: 5\n", "max_media"},
		"bad duration":        {"hosting:\n  upload_timeout: soon\n", "parsing config file"},
		"unitless duration":   {"graph_api:\n  timeout: 30\n", "time.Duration"},
		"zero concurrency":    {"insights:\n  concurrency: 0\n", "insights.concurrency"},
		"default above max":   {"tools:\n  max_media_limit: 20\n", "tools.reels_default"},
		"negative timeout":    {"graph_api:\n  timeout: -1s\n", "graph_api.timeout"},
		"empty release tag":   {"hosting:\n  release_tag: \"\"\n", "hosting.release_tag"},
		"secrets not in file": {"access_token: x\n", "access_token"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load("", writeConfig(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}

	if _, err := config.Load("", filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoad_EmptyConfigFile(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")

	cfg, err := config.Load("", writeConfig(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tools.MaxMediaLimit != 500 {
		t.Errorf("expected default max media limit, got %d", cfg.Tools.MaxMediaLimit)
	}
}

// TestExampleConfigMatchesDefaults keeps config.example.yaml in sync with Default().
func TestExampleConfigMatchesDefaults(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")

	cfg, err := config.Load("", "../../config.example.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := config.Default()
	if cfg.GraphAPI != want.GraphAPI || cfg.Polling != want.Polling || cfg.Insights != want.Insights ||
		cfg.Tools != want.Tools || cfg.Hosting != want.Hosting {
		t.Errorf("config.example.yaml drifted from Default():\n got %+v\nwant %+v", cfg, want)
	}
}

func TestOptionsFromConfig(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	path := writeConfig(t, `
insights:
  concurrency: 3
tools:
  max_media_limit: 100
  list_media_default: 7
  reels_default: 9
  cleanup_older_than: 2h
hosting:
  upload_timeout: 5m
  request_timeout: 10s
  release_tag: tag-x
`)

	cfg, err := config.Load("", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	h := cfg.HostingOptions()
	if h.UploadTimeout != 5*time.Minute || h.RequestTimeout != 10*time.Second || h.ReleaseTag != "tag-x" {
		t.Errorf("unexpected hosting options: %+v", h)
	}
	o := cfg.ToolsOptions()
	if o.MaxMediaLimit != 100 || o.ListMediaDefault != 7 || o.ReelsDefault != 9 ||
		o.InsightsConcurrency != 3 || o.CleanupOlderThan != 2*time.Hour {
		t.Errorf("unexpected tools options: %+v", o)
	}
}
