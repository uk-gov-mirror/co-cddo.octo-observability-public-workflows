package ingestion_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/co-cddo/octo-observability-public-workflows/internal/ingestion"
	"github.com/co-cddo/octo-observability-public-workflows/internal/logger"
)

const (
	testServiceID = "123e4567-e89b-12d3-a456-426614174000"
	testAPIKey    = "secret-api-key-xyz789"
)

func newTestLogger() *logger.Logger {
	return logger.New([]string{testAPIKey})
}

func setupIngestionServer(t *testing.T, statusCode int, responseBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != testAPIKey {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized"))
			return
		}
		if r.Header.Get("Content-Type") != "application/spdx+json" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad content-type"))
			return
		}
		w.WriteHeader(statusCode)
		if responseBody != "" {
			w.Write([]byte(responseBody))
		}
	}))
	return srv
}

func newClient(t *testing.T, serverURL string) *ingestion.Client {
	t.Helper()
	return ingestion.NewClient(serverURL, testAPIKey, newTestLogger())
}

func TestSubmit_success200(t *testing.T) {
	srv := setupIngestionServer(t, http.StatusOK, "")
	defer srv.Close()

	client := newClient(t, srv.URL)
	result, err := client.Submit(context.Background(), testServiceID, []byte(`{"spdxVersion":"SPDX-2.3"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success=true, got status=%d error=%q", result.StatusCode, result.ErrorMessage)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
}

func TestSubmit_success201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := ingestion.NewClient(srv.URL, testAPIKey, newTestLogger())
	result, err := client.Submit(context.Background(), testServiceID, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected 201 to be success")
	}
}

func TestSubmit_unauthorised_sanitisesAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate an API that echoes back the API key in the error body
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid key: " + testAPIKey))
	}))
	defer srv.Close()

	client := ingestion.NewClient(srv.URL, testAPIKey, newTestLogger())
	result, err := client.Submit(context.Background(), testServiceID, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected success=false")
	}
	if result.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", result.StatusCode)
	}
	if strings.Contains(result.ErrorMessage, testAPIKey) {
		t.Errorf("API key must be redacted from error message, got: %q", result.ErrorMessage)
	}
	if !strings.Contains(result.ErrorMessage, "***") {
		t.Errorf("expected *** redaction in error message, got: %q", result.ErrorMessage)
	}
}

func TestSubmit_badRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid payload"))
	}))
	defer srv.Close()

	client := ingestion.NewClient(srv.URL, testAPIKey, newTestLogger())
	result, err := client.Submit(context.Background(), testServiceID, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected success=false")
	}
	if result.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", result.StatusCode)
	}
	if result.ErrorMessage == "" {
		t.Error("expected non-empty error message")
	}
}

func TestSubmit_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	client := ingestion.NewClient(srv.URL, testAPIKey, newTestLogger())
	result, err := client.Submit(context.Background(), testServiceID, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected success=false for 500")
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", result.StatusCode)
	}
}

func TestSubmit_setsRequiredHeaders(t *testing.T) {
	var capturedAPIKey, capturedContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("X-API-Key")
		capturedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := ingestion.NewClient(srv.URL, testAPIKey, newTestLogger())
	_, err := client.Submit(context.Background(), testServiceID, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedAPIKey != testAPIKey {
		t.Errorf("X-API-Key header: got %q, want %q", capturedAPIKey, testAPIKey)
	}
	if capturedContentType != "application/spdx+json" {
		t.Errorf("Content-Type header: got %q, want application/spdx+json", capturedContentType)
	}
}

func TestSubmit_urlContainsServiceID(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := ingestion.NewClient(srv.URL, testAPIKey, newTestLogger())
	_, err := client.Submit(context.Background(), testServiceID, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "/api/modules/sbom/services/" + testServiceID
	if capturedPath != expected {
		t.Errorf("expected path %q, got %q", expected, capturedPath)
	}
}

func TestSubmit_contextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := ingestion.NewClient(srv.URL, testAPIKey, newTestLogger())
	_, err := client.Submit(ctx, testServiceID, []byte(`{}`))
	if err == nil {
		t.Error("expected error due to cancelled context")
	}
}
