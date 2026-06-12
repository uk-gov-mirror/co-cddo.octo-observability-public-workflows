package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/co-cddo/octo-observability-public-workflows/internal/logger"
)

const (
	apiBase    = "https://api.github.com"
	apiVersion = "2022-11-28"
)

// Client retrieves SBOMs from GitHub's dependency graph API.
type Client struct {
	token      string
	httpClient *http.Client
	log        *logger.Logger
}

// NewClient creates a GitHub Client with the provided token and logger.
func NewClient(token string, log *logger.Logger) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// NewClientWithTransport creates a GitHub Client with a custom HTTP transport.
// Intended for testing only — use NewClient in production.
func NewClientWithTransport(token string, log *logger.Logger, transport http.RoundTripper) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		log: log,
	}
}

// FetchSBOM retrieves the SBOM for the given owner/repo combination.
// It triggers async report generation, then polls until the report is ready.
func (c *Client) FetchSBOM(ctx context.Context, owner, repo string, pollInterval time.Duration, maxAttempts int) ([]byte, error) {
	sbomURL, err := c.requestReport(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	// Extract the UUID from the last path segment of sbomURL.
	parts := strings.Split(strings.TrimRight(sbomURL, "/"), "/")
	uuid := parts[len(parts)-1]
	if uuid == "" {
		return nil, fmt.Errorf("could not extract report UUID from sbom_url: %s", sbomURL)
	}

	return c.pollReport(ctx, owner, repo, uuid, pollInterval, maxAttempts)
}

// requestReport triggers async SBOM generation and returns the sbom_url from the response.
func (c *Client) requestReport(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/dependency-graph/sbom/generate-report", apiBase, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("generate-report request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read generate-report response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		// success — parse sbom_url
	case http.StatusForbidden:
		return "", fmt.Errorf("insufficient permissions — ensure token has contents:read and dependency graph is enabled")
	case http.StatusNotFound:
		return "", fmt.Errorf("dependency graph not available — enable it in repository settings")
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("GitHub API rate limit exceeded — try again later")
	default:
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		SBOMURL string `json:"sbom_url"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// Some GitHub responses return the SBOM URL as plain text
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "http") {
			return trimmed, nil
		}
		return "", fmt.Errorf("parse generate-report response: %w", err)
	}
	if result.SBOMURL == "" {
		return "", fmt.Errorf("generate-report response missing sbom_url field")
	}
	return result.SBOMURL, nil
}

// pollReport polls the fetch-report endpoint until the SBOM is ready.
// Transient failures (network errors, 5xx) are retried within the poll loop.
// Persistent 4xx errors abort immediately.
func (c *Client) pollReport(ctx context.Context, owner, repo, uuid string, interval time.Duration, maxAttempts int) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/dependency-graph/sbom/fetch-report/%s", apiBase, owner, repo, uuid)

	consecutiveErrors := 0
	const maxConsecutiveErrors = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create poll request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			consecutiveErrors++
			c.log.Info("poll request failed, will retry",
				logger.F("attempt", fmt.Sprintf("%d/%d", attempt, maxAttempts)),
				logger.F("error", err.Error()),
			)
			if consecutiveErrors >= maxConsecutiveErrors {
				return nil, fmt.Errorf("fetch-report failed %d consecutive times: %w", maxConsecutiveErrors, err)
			}
		} else {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return body, nil
			}

			// 4xx (except 429) are definitive failures — abort
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
				return nil, fmt.Errorf("fetch-report returned HTTP %d", resp.StatusCode)
			}

			if readErr != nil {
				consecutiveErrors++
			} else {
				consecutiveErrors = 0
			}

			if consecutiveErrors >= maxConsecutiveErrors {
				return nil, fmt.Errorf("fetch-report returned persistent errors after %d attempts", maxConsecutiveErrors)
			}
		}

		c.log.Info("SBOM report generating",
			logger.F("attempt", fmt.Sprintf("%d/%d", attempt, maxAttempts)),
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}

	return nil, fmt.Errorf("timed out waiting for SBOM report after %d attempts", maxAttempts)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
}
