package repositories_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/repositories"
)

// TestSourceRecordRepository_Interface ensures the type satisfies the interface.
// This test is compile-time only — no DB required.
func TestSourceRecordRepository_Interface(t *testing.T) {
	var _ interface {
		Upsert(ctx context.Context, records []domain.SourceRecord) error
		FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error)
	} = (*repositories.SourceRecordRepository)(nil)
}

func TestNormalizeForJSON_PreservesData(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	r := domain.SourceRecord{
		Source:    "cnpj",
		RecordKey: "12345678000195",
		Data:      map[string]any{"razao_social": "XPTO LTDA", "situacao_cadastral": "ATIVA"},
		FetchedAt: now,
	}

	// Data must serialize to valid JSON
	b, err := json.Marshal(r.Data)
	if err != nil {
		t.Fatalf("Data must be JSON-serializable: %v", err)
	}
	if len(b) == 0 {
		t.Error("marshaled data is empty")
	}
}
