package logger

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// Field is a key-value pair for structured log output.
type Field struct {
	Key   string
	Value string
}

// F constructs a Field.
func F(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Logger writes JSON-structured log lines to stdout with secret redaction.
type Logger struct {
	secrets []string
}

// New creates a Logger that redacts all values in secrets from every log line.
func New(secrets []string) *Logger {
	filtered := make([]string, 0, len(secrets))
	for _, s := range secrets {
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	return &Logger{secrets: filtered}
}

// Info emits an info-level JSON log line.
func (l *Logger) Info(msg string, fields ...Field) {
	l.emit("info", msg, fields)
}

// Error emits an error-level JSON log line.
func (l *Logger) Error(msg string, fields ...Field) {
	l.emit("error", msg, fields)
}

// Redact replaces all registered secrets in s with ***.
func (l *Logger) Redact(s string) string {
	for _, secret := range l.secrets {
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}

func (l *Logger) emit(level, msg string, fields []Field) {
	entry := map[string]string{
		"level": level,
		"msg":   l.Redact(msg),
		"time":  time.Now().UTC().Format(time.RFC3339),
	}
	for _, f := range fields {
		entry[f.Key] = l.Redact(f.Value)
	}
	data, _ := json.Marshal(entry)
	os.Stdout.Write(append(data, '\n'))
}
