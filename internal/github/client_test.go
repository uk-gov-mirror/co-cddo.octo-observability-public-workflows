package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ghclient "github.com/co-cddo/octo-observability-public-workflows/internal/github"
	"github.com/co-cddo/octo-observability-public-workflows/internal/logger"
)

const testUUID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

// newTestLogger returns a logger that redacts the test token.
func newTestLogger() *logger.Logger {
	return logger.New([]string{"secret-token"})
}

// buildTestServer creates a fake GitHub API server.
// generateStatus controls the HTTP status of the generate-report endpoint.
// fetchHandler is called for each fetch-report poll request.
func buildTestServer(t *testing.T, generateStatus int, fetchHandler http.HandlerFunc) *httptest.Server {
	t.Helper()

	var sbomURL string
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/org/repo/dependency-graph/sbom/generate-report", func(w http.ResponseWriter, r *http.Request) {
		if generateStatus != http.StatusOK {
			w.WriteHeader(generateStatus)
			return
		}
		resp, _ := json.Marshal(map[string]string{"sbom_url": sbomURL})
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	})

	mux.HandleFunc(
		fmt.Sprintf("/repos/org/repo/dependency-graph/sbom/fetch-report/%s", testUUID),
		fetchHandler,
	)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	sbomURL = fmt.Sprintf("%s/repos/org/repo/dependency-graph/sbom/fetch-report/%s", srv.URL, testUUID)

	return srv
}

// rewriteTransport redirects api.github.com calls to the test server.
type rewriteTransport struct {
	target string // e.g. "http://127.0.0.1:PORT"
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = strings.TrimPrefix(rt.target, "http://")
	return http.DefaultTransport.RoundTrip(req2)
}

// newClientAgainst creates a real GitHub client whose HTTP calls are redirected to srv.
func newClientAgainst(srv *httptest.Server) *ghclient.Client {
	transport := &rewriteTransport{target: srv.URL}
	return ghclient.NewClientWithTransport("secret-token", newTestLogger(), transport)
}

// --- Tests ---

func TestFetchSBOM_success(t *testing.T) {
	expectedBody := `{"spdxVersion":"SPDX-2.3"}`
	srv := buildTestServer(t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	})

	client := newClientAgainst(srv)
	got, err := client.FetchSBOM(context.Background(), "org", "repo", 1*time.Millisecond, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != expectedBody {
		t.Errorf("got body %q, want %q", string(got), expectedBody)
	}
}

func TestFetchSBOM_permissionsError(t *testing.T) {
	srv := buildTestServer(t, http.StatusForbidden, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // never reached
	})

	client := newClientAgainst(srv)
	_, err := client.FetchSBOM(context.Background(), "org", "repo", 1*time.Millisecond, 5)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "insufficient permissions") {
		t.Errorf("expected permissions error message, got: %v", err)
	}
}

func TestFetchSBOM_dependencyGraphNotEnabled(t *testing.T) {
	srv := buildTestServer(t, http.StatusNotFound, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // never reached
	})

	client := newClientAgainst(srv)
	_, err := client.FetchSBOM(context.Background(), "org", "repo", 1*time.Millisecond, 5)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "dependency graph not available") {
		t.Errorf("expected dependency graph error, got: %v", err)
	}
}

func TestFetchSBOM_rateLimited(t *testing.T) {
	srv := buildTestServer(t, http.StatusTooManyRequests, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // never reached
	})

	client := newClientAgainst(srv)
	_, err := client.FetchSBOM(context.Background(), "org", "repo", 1*time.Millisecond, 5)
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestFetchSBOM_pollTimeout(t *testing.T) {
	// Fetch endpoint always returns 202 (not ready yet).
	srv := buildTestServer(t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // never 200
	})

	client := newClientAgainst(srv)
	_, err := client.FetchSBOM(context.Background(), "org", "repo", 1*time.Millisecond, 3)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestFetchSBOM_pollSucceedsAfterRetries(t *testing.T) {
	callCount := 0
	finalBody := `{"spdxVersion":"SPDX-2.3"}`
	srv := buildTestServer(t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusAccepted) // not ready yet
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(finalBody))
	})

	client := newClientAgainst(srv)
	got, err := client.FetchSBOM(context.Background(), "org", "repo", 1*time.Millisecond, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != finalBody {
		t.Errorf("got %q, want %q", string(got), finalBody)
	}
}

func TestFetchSBOM_contextCancellation(t *testing.T) {
	srv := buildTestServer(t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := newClientAgainst(srv)
	_, err := client.FetchSBOM(ctx, "org", "repo", 1*time.Millisecond, 10)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchSBOM_tokenNotInLogs(t *testing.T) {
	token := "super-secret-github-token"
	log := logger.New([]string{token})

	srv := buildTestServer(t, http.StatusForbidden, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	transport := &rewriteTransport{target: srv.URL}
	client := ghclient.NewClientWithTransport(token, log, transport)

	_, err := client.FetchSBOM(context.Background(), "org", "repo", 1*time.Millisecond, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	// Error message itself should not contain the token
	if strings.Contains(err.Error(), token) {
		t.Errorf("token must not appear in error message")
	}
}
