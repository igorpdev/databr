package ibge_test

import (
	"context"
	"testing"

	"github.com/databr/api/internal/collectors/ibge"
)

var fakePIBResponse = []map[string]any{
	{
		"NC":  "1",
		"NN":  "Brasil",
		"V":   "2800000",
		"D2C": "202503",
		"D2N": "3º trimestre 2025",
		"D3C": "6561",
		"D3N": "PIB",
	},
}

func TestPIBCollector_Source(t *testing.T) {
	srv := newSIDRAServer(t, fakePIBResponse)
	defer srv.Close()
	c := ibge.NewPIBCollector(srv.URL)
	if c.Source() != "ibge_pib" {
		t.Errorf("Source() = %q, want ibge_pib", c.Source())
	}
}

func TestPIBCollector_Schedule(t *testing.T) {
	srv := newSIDRAServer(t, fakePIBResponse)
	defer srv.Close()
	c := ibge.NewPIBCollector(srv.URL)
	if c.Schedule() != "0 8 * * *" {
		t.Errorf("Schedule() = %q, want 0 8 * * *", c.Schedule())
	}
}

func TestPIBCollector_Collect(t *testing.T) {
	srv := newSIDRAServer(t, fakePIBResponse)
	defer srv.Close()

	c := ibge.NewPIBCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	r := records[0]
	if r.Source != "ibge_pib" {
		t.Errorf("Source = %q, want ibge_pib", r.Source)
	}
	for _, field := range []string{"periodo", "valor", "indicador"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing normalized field %q", field)
		}
	}
}
