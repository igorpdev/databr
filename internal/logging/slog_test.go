package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/databr/api/internal/logging"
)

func TestSetup_JSON_InProduction(t *testing.T) {
	t.Setenv("RAILWAY_ENVIRONMENT", "production")
	var buf bytes.Buffer
	logging.Setup(&buf)

	slog.Info("test message", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected JSON log output, got: %s", buf.String())
	}
	if entry["msg"] != "test message" {
		t.Errorf("msg = %v, want 'test message'", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("key = %v, want 'value'", entry["key"])
	}
}

func TestSetup_Text_InDev(t *testing.T) {
	os.Unsetenv("RAILWAY_ENVIRONMENT")
	os.Unsetenv("FLY_APP_NAME")
	var buf bytes.Buffer
	logging.Setup(&buf)

	slog.Info("dev message")

	if !bytes.Contains(buf.Bytes(), []byte("dev message")) {
		t.Errorf("expected text output containing 'dev message', got: %s", buf.String())
	}
}
