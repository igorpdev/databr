package juridico_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/juridico"
)

var fakeDataJudResponse = map[string]any{
	"hits": map[string]any{
		"total": map[string]any{"value": 1},
		"hits": []map[string]any{
			{
				"_source": map[string]any{
					"numeroProcesso": "0001234-56.2023.8.26.0001",
					"tribunal":       "TJSP",
					"classe":         map[string]any{"nome": "Ação Civil Pública"},
				},
			},
		},
	},
}

func newDataJudServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeDataJudResponse)
	}))
}

func TestDataJudCollector_Source(t *testing.T) {
	srv := newDataJudServer(t)
	defer srv.Close()
	c := juridico.NewDataJudCollector(srv.URL, "test-key")
	if c.Source() != "datajud_cnj" {
		t.Errorf("Source() = %q, want datajud_cnj", c.Source())
	}
}

func TestDataJudCollector_Search(t *testing.T) {
	srv := newDataJudServer(t)
	defer srv.Close()
	c := juridico.NewDataJudCollector(srv.URL, "test-key")
	records, err := c.Search(context.Background(), "12345678909")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].Source != "datajud_cnj" {
		t.Errorf("Source = %q, want datajud_cnj", records[0].Source)
	}
}

func TestParseCNJNumber(t *testing.T) {
	tests := []struct {
		input     string
		wantIndex string
		wantClean string
		wantErr   bool
	}{
		// J=4 Federal TR=01 → trf1
		{"0000832-35.2018.4.01.3202", "api_publica_trf1", "00008323520184013202", false},
		// J=8 Estadual TR=26=SP → tjsp
		{"1234567-89.2023.8.26.0001", "api_publica_tjsp", "12345678920238260001", false},
		// J=5 Trabalho TR=02 → trt2
		{"0001234-56.2024.5.02.0000", "api_publica_trt2", "00012345620245020000", false},
		// J=6 Eleitoral TR=26=SP → tre-sp (with hyphen)
		{"0001234-56.2024.6.26.0000", "api_publica_tre-sp", "00012345620246260000", false},
		// J=9 Militar estadual TR=13=MG → tjmmg
		{"0001234-56.2024.9.13.0000", "api_publica_tjmmg", "00012345620249130000", false},
		// J=3 STJ (Superior Tribunal de Justiça)
		{"0001234-56.2024.3.00.0000", "api_publica_stj", "00012345620243000000", false},
		// J=7 STM (Superior Tribunal Militar)
		{"0001234-56.2024.7.00.0000", "api_publica_stm", "00012345620247000000", false},
		// J=5 TST (TR=00 → superior court)
		{"0001234-56.2024.5.00.0000", "api_publica_tst", "00012345620245000000", false},
		// J=6 TSE (TR=00 → superior court)
		{"0001234-56.2024.6.00.0000", "api_publica_tse", "00012345620246000000", false},
		// Unformatted 20-digit number (J=4 federal TR=01)
		{"00008323520184013202", "api_publica_trf1", "00008323520184013202", false},
		// J=1 STF — not available in DataJud
		{"0001234-56.2024.1.00.0000", "", "", true},
		// Invalid input
		{"invalid", "", "", true},
		// Too short
		{"12345", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			index, clean, err := juridico.ParseCNJNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCNJNumber(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if index != tt.wantIndex {
				t.Errorf("ParseCNJNumber(%q) index = %q, want %q", tt.input, index, tt.wantIndex)
			}
			if clean != tt.wantClean {
				t.Errorf("ParseCNJNumber(%q) clean = %q, want %q", tt.input, clean, tt.wantClean)
			}
		})
	}
}

func TestDataJudCollector_SearchByNumber(t *testing.T) {
	srv := newDataJudServer(t)
	defer srv.Close()
	c := juridico.NewDataJudCollector(srv.URL, "test-key")

	records, err := c.SearchByNumber(context.Background(), "0000832-35.2018.4.01.3202")
	if err != nil {
		t.Fatalf("SearchByNumber() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].Source != "datajud_cnj" {
		t.Errorf("Source = %q, want datajud_cnj", records[0].Source)
	}
}

func TestDataJudCollector_SearchByNumber_Invalid(t *testing.T) {
	c := juridico.NewDataJudCollector("", "test-key")
	_, err := c.SearchByNumber(context.Background(), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}
