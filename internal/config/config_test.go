package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/co-cddo/octo-observability-public-workflows/internal/config"
)

var validEnv = []string{
	"SBOM_REPO=org/repo",
	"SBOM_GITHUB_TOKEN=ghp_token",
	"SBOM_API_KEY=secret-key",
	"SBOM_API_URL=https://ingest.example.com",
	"SBOM_SERVICE_ID=123e4567-e89b-12d3-a456-426614174000",
}

func TestParse_envVarsFallback(t *testing.T) {
	cfg, err := config.Parse([]string{}, validEnv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Repo != "org/repo" {
		t.Errorf("Repo: got %q, want %q", cfg.Repo, "org/repo")
	}
	if cfg.GitHubToken != "ghp_token" {
		t.Errorf("GitHubToken: got %q", cfg.GitHubToken)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval default: got %v", cfg.PollInterval)
	}
	if cfg.PollMaxAttempts != 24 {
		t.Errorf("PollMaxAttempts default: got %d", cfg.PollMaxAttempts)
	}
}

func TestParse_flagsOverrideEnv(t *testing.T) {
	args := []string{"--repo", "flag-org/flag-repo", "--github-token", "flag-token"}
	env := []string{"SBOM_REPO=env-org/env-repo", "SBOM_GITHUB_TOKEN=env-token"}
	cfg, err := config.Parse(args, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Repo != "flag-org/flag-repo" {
		t.Errorf("flag should override env for Repo: got %q", cfg.Repo)
	}
	if cfg.GitHubToken != "flag-token" {
		t.Errorf("flag should override env for GitHubToken: got %q", cfg.GitHubToken)
	}
}

func TestParse_pollIntervalFromEnv(t *testing.T) {
	env := append(validEnv, "SBOM_POLL_INTERVAL=10s")
	cfg, err := config.Parse([]string{}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("PollInterval: got %v, want 10s", cfg.PollInterval)
	}
}

func TestParse_pollMaxAttemptsFromFlag(t *testing.T) {
	cfg, err := config.Parse([]string{"--poll-max-attempts", "48"}, validEnv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PollMaxAttempts != 48 {
		t.Errorf("PollMaxAttempts: got %d, want 48", cfg.PollMaxAttempts)
	}
}

func TestParse_versionFlag(t *testing.T) {
	cfg, err := config.Parse([]string{"--version"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Version {
		t.Error("expected Version=true")
	}
}

func TestValidate_valid(t *testing.T) {
	cfg := &config.Config{
		Repo:            "org/repo",
		GitHubToken:     "ghp_token",
		APIKey:          "key",
		APIURL:          "https://ingest.example.com",
		ServiceID:       "123e4567-e89b-12d3-a456-426614174000",
		PollInterval:    5 * time.Second,
		PollMaxAttempts: 24,
	}
	result := config.Validate(cfg)
	if !result.Valid {
		t.Errorf("expected valid config, got errors: %v", result.Errors)
	}
}

func TestValidate_collectsAllErrors(t *testing.T) {
	cfg := &config.Config{}
	result := config.Validate(cfg)
	if result.Valid {
		t.Fatal("expected invalid config")
	}
	// Expect errors for all 5 required fields
	if len(result.Errors) < 5 {
		t.Errorf("expected at least 5 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestValidate_missingRepo(t *testing.T) {
	cfg := &config.Config{GitHubToken: "t", APIKey: "k", APIURL: "https://x.com", ServiceID: "123e4567-e89b-12d3-a456-426614174000"}
	result := config.Validate(cfg)
	if result.Valid {
		t.Fatal("expected invalid")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "repo") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected repo error, got: %v", result.Errors)
	}
}

func TestValidate_invalidRepoFormat(t *testing.T) {
	cfg := &config.Config{Repo: "justname", GitHubToken: "t", APIKey: "k", APIURL: "https://x.com", ServiceID: "123e4567-e89b-12d3-a456-426614174000"}
	result := config.Validate(cfg)
	if result.Valid {
		t.Fatal("expected invalid")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "owner/repo") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected format error, got: %v", result.Errors)
	}
}

func TestValidate_httpURLAllowed(t *testing.T) {
	cfg := &config.Config{
		Repo: "org/repo", GitHubToken: "t", APIKey: "k",
		APIURL:    "http://ingest.internal",
		ServiceID: "123e4567-e89b-12d3-a456-426614174000",
	}
	result := config.Validate(cfg)
	if !result.Valid {
		t.Errorf("http:// URL should be allowed, got errors: %v", result.Errors)
	}
}

func TestValidate_invalidURL(t *testing.T) {
	cfg := &config.Config{
		Repo: "org/repo", GitHubToken: "t", APIKey: "k",
		APIURL:    "ftp://bad.com",
		ServiceID: "123e4567-e89b-12d3-a456-426614174000",
	}
	result := config.Validate(cfg)
	if result.Valid {
		t.Fatal("expected invalid URL")
	}
}

func TestValidate_invalidUUID(t *testing.T) {
	cfg := &config.Config{
		Repo: "org/repo", GitHubToken: "t", APIKey: "k",
		APIURL:    "https://x.com",
		ServiceID: "not-a-uuid",
	}
	result := config.Validate(cfg)
	if result.Valid {
		t.Fatal("expected invalid UUID")
	}
}

func TestValidate_uppercaseUUID(t *testing.T) {
	cfg := &config.Config{
		Repo: "org/repo", GitHubToken: "t", APIKey: "k",
		APIURL:    "https://x.com",
		ServiceID: "123E4567-E89B-12D3-A456-426614174000",
	}
	result := config.Validate(cfg)
	if !result.Valid {
		t.Errorf("uppercase UUID should be valid, got errors: %v", result.Errors)
	}
}

