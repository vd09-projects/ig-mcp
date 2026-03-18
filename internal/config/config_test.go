package config_test

import (
	"os"
	"testing"

	"github.com/vikrant/instagram-mcp/internal/config"
)

func TestLoad_MissingRequiredFields(t *testing.T) {
	os.Unsetenv("INSTAGRAM_ACCESS_TOKEN")
	os.Unsetenv("INSTAGRAM_ACCOUNT_ID")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when required env vars are missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "test-token")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "123456")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GraphAPIVersion != "v21.0" {
		t.Errorf("expected default API version v21.0, got %s", cfg.GraphAPIVersion)
	}
	if cfg.PollMaxAttempts != 60 {
		t.Errorf("expected default poll max 60, got %d", cfg.PollMaxAttempts)
	}
	if cfg.BaseURL() != "https://graph.facebook.com/v21.0" {
		t.Errorf("unexpected base URL: %s", cfg.BaseURL())
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	t.Setenv("GRAPH_API_VERSION", "v22.0")
	t.Setenv("STATUS_POLL_INTERVAL_MS", "2000")
	t.Setenv("STATUS_POLL_MAX_ATTEMPTS", "10")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GraphAPIVersion != "v22.0" {
		t.Errorf("expected v22.0, got %s", cfg.GraphAPIVersion)
	}
	if cfg.PollMaxAttempts != 10 {
		t.Errorf("expected 10, got %d", cfg.PollMaxAttempts)
	}
}

func TestLoad_InvalidPollInterval(t *testing.T) {
	t.Setenv("INSTAGRAM_ACCESS_TOKEN", "tok")
	t.Setenv("INSTAGRAM_ACCOUNT_ID", "999")
	t.Setenv("STATUS_POLL_INTERVAL_MS", "not-a-number")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for non-numeric poll interval")
	}
}
