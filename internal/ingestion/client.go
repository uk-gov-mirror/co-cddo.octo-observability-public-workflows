package ingestion

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/co-cddo/octo-observability-public-workflows/internal/logger"
)

// SubmissionResult holds the outcome of an SBOM submission.
type SubmissionResult struct {
	Success      bool
	StatusCode   int
	ErrorMessage string
}

// Client submits SBOMs to the observability platform's ingestion API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	log        *logger.Logger
}

// NewClient creates an ingestion Client with the given base URL, API key, and logger.
func NewClient(baseURL, apiKey string, log *logger.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// NewClientWithTransport creates a Client with a custom HTTP transport.
// Intended for testing only — use NewClient in production.
func NewClientWithTransport(baseURL, apiKey string, log *logger.Logger, transport http.RoundTripper) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		log: log,
	}
}

// Submit posts the SBOM payload to the ingestion API for the given serviceID.
func (c *Client) Submit(ctx context.Context, serviceID string, sbom []byte) (*SubmissionResult, error) {
	url := fmt.Sprintf("%s/api/modules/sbom/services/%s", c.baseURL, serviceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(sbom))
	if err != nil {
		return nil, fmt.Errorf("create submission request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/spdx+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submission request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read submission response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &SubmissionResult{
			Success:    true,
			StatusCode: resp.StatusCode,
		}, nil
	}

	// Sanitise response body — the API may echo back our credentials in error messages.
	sanitised := strings.ReplaceAll(string(body), c.apiKey, "***")

	return &SubmissionResult{
		Success:      false,
		StatusCode:   resp.StatusCode,
		ErrorMessage: sanitised,
	}, nil
}
