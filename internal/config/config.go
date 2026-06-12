package config

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Config holds all runtime configuration for the sbom-retriever.
type Config struct {
	Repo            string
	GitHubToken     string
	APIKey          string
	APIURL          string
	ServiceID       string
	PollInterval    time.Duration
	PollMaxAttempts int
	Version         bool
}

// ValidationResult holds the outcome of Config validation.
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// Parse reads configuration from args (CLI flags) and environ (env vars).
// Flags take precedence over environment variables.
func Parse(args []string, environ []string) (*Config, error) {
	env := parseEnviron(environ)

	fs := flag.NewFlagSet("sbom-retriever", flag.ContinueOnError)

	repo := fs.String("repo", "", "GitHub repository in owner/repo format")
	githubToken := fs.String("github-token", "", "GitHub personal access token")
	apiKey := fs.String("api-key", "", "Ingestion API key")
	apiURL := fs.String("api-url", "", "Ingestion API base URL")
	serviceID := fs.String("service-id", "", "Service UUID for SBOM submission")
	pollInterval := fs.String("poll-interval", "", "Polling interval (e.g. 5s)")
	pollMaxAttempts := fs.Int("poll-max-attempts", 0, "Maximum polling attempts")
	version := fs.Bool("version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("flag parsing: %w", err)
	}

	cfg := &Config{
		Repo:        coalesce(*repo, env["SBOM_REPO"]),
		GitHubToken: coalesce(*githubToken, env["SBOM_GITHUB_TOKEN"]),
		APIKey:      coalesce(*apiKey, env["SBOM_API_KEY"]),
		APIURL:      coalesce(*apiURL, env["SBOM_API_URL"]),
		ServiceID:   coalesce(*serviceID, env["SBOM_SERVICE_ID"]),
		Version:     *version,
	}

	// PollInterval: flag → env → default 5s
	if *pollInterval != "" {
		d, err := time.ParseDuration(*pollInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid --poll-interval %q: %w", *pollInterval, err)
		}
		cfg.PollInterval = d
	} else if v := env["SBOM_POLL_INTERVAL"]; v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SBOM_POLL_INTERVAL %q: %w", v, err)
		}
		cfg.PollInterval = d
	} else {
		cfg.PollInterval = 5 * time.Second
	}

	// PollMaxAttempts: flag → env → default 24
	if *pollMaxAttempts != 0 {
		cfg.PollMaxAttempts = *pollMaxAttempts
	} else if v := env["SBOM_POLL_MAX_ATTEMPTS"]; v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SBOM_POLL_MAX_ATTEMPTS %q: %w", v, err)
		}
		cfg.PollMaxAttempts = n
	} else {
		cfg.PollMaxAttempts = 24
	}

	return cfg, nil
}

// Validate checks all required fields and returns a ValidationResult.
// All errors are collected before returning — validation does not fail fast.
func Validate(cfg *Config) ValidationResult {
	var errs []string

	if cfg.Repo == "" {
		errs = append(errs, "missing required parameter: --repo (or SBOM_REPO)")
	} else if strings.Count(cfg.Repo, "/") != 1 {
		errs = append(errs, "invalid --repo: must be in owner/repo format (e.g. myorg/myrepo)")
	}

	if cfg.GitHubToken == "" {
		errs = append(errs, "missing required parameter: --github-token (or SBOM_GITHUB_TOKEN)")
	}

	if cfg.APIKey == "" {
		errs = append(errs, "missing required parameter: --api-key (or SBOM_API_KEY)")
	}

	if cfg.APIURL == "" {
		errs = append(errs, "missing required parameter: --api-url (or SBOM_API_URL)")
	} else if !strings.HasPrefix(cfg.APIURL, "http://") && !strings.HasPrefix(cfg.APIURL, "https://") {
		errs = append(errs, "invalid --api-url: must start with http:// or https://")
	}

	if cfg.ServiceID == "" {
		errs = append(errs, "missing required parameter: --service-id (or SBOM_SERVICE_ID)")
	} else if !uuidRegex.MatchString(cfg.ServiceID) {
		errs = append(errs, "invalid --service-id: must be a UUID (e.g. 123e4567-e89b-12d3-a456-426614174000)")
	}

	return ValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	}
}

// coalesce returns the first non-empty string.
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseEnviron converts a []string of "KEY=VALUE" pairs to a map.
func parseEnviron(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, e := range environ {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}
