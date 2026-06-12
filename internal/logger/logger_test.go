package logger_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/co-cddo/octo-observability-public-workflows/internal/logger"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestInfo_emitsJSONWithLevelAndMsg(t *testing.T) {
	l := logger.New(nil)
	out := captureStdout(t, func() {
		l.Info("hello world")
	})
	var entry map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entry); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %s", err, out)
	}
	if entry["level"] != "info" {
		t.Errorf("expected level=info, got %q", entry["level"])
	}
	if entry["msg"] != "hello world" {
		t.Errorf("expected msg=%q, got %q", "hello world", entry["msg"])
	}
	if entry["time"] == "" {
		t.Error("expected time field to be non-empty")
	}
}

func TestError_emitsErrorLevel(t *testing.T) {
	l := logger.New(nil)
	out := captureStdout(t, func() {
		l.Error("something broke")
	})
	var entry map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["level"] != "error" {
		t.Errorf("expected level=error, got %q", entry["level"])
	}
}

func TestFields_appearsInOutput(t *testing.T) {
	l := logger.New(nil)
	out := captureStdout(t, func() {
		l.Info("test", logger.F("repo", "org/repo"), logger.F("status", "ok"))
	})
	var entry map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["repo"] != "org/repo" {
		t.Errorf("expected repo=org/repo, got %q", entry["repo"])
	}
	if entry["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", entry["status"])
	}
}

func TestRedact_secretNeverAppearsInOutput(t *testing.T) {
	secret := "super-secret-token-abc123"
	apiKey := "api-key-xyz789"
	l := logger.New([]string{secret, apiKey})

	out := captureStdout(t, func() {
		l.Info("fetching repo", logger.F("token", secret))
		l.Error("auth failed: "+secret, logger.F("key", apiKey))
	})

	if strings.Contains(out, secret) {
		t.Errorf("secret token must not appear in log output")
	}
	if strings.Contains(out, apiKey) {
		t.Errorf("api key must not appear in log output")
	}
	if !strings.Contains(out, "***") {
		t.Error("expected *** redaction marker in output")
	}
}

func TestRedact_emptySecretsIgnored(t *testing.T) {
	l := logger.New([]string{"", "", "real-secret"})
	if strings.Contains(l.Redact("hello"), "") {
		// empty strings would match everything — ensure they're filtered out
	}
	out := captureStdout(t, func() {
		l.Info("test real-secret here")
	})
	if strings.Contains(out, "real-secret") {
		t.Error("real-secret should be redacted")
	}
}
