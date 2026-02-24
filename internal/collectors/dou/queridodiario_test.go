package dou_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/collectors/dou"
)

var fakeGazetteResponse = map[string]any{
	"gazettes": []map[string]any{
		{
			"territory_id":   "3550308",
			"territory_name": "São Paulo",
			"state_code":     "SP",
			"date":           "2026-02-01",
			"url":            "https://queridodiario.ok.org.br/diario",
			"excerpts":       []string{"contrato de licitação..."},
		},
	},
	"total_gazettes": 1,
}

var fakeCitiesResponse = map[string]any{
	"cities": []map[string]any{
		{
			"territory_id":     "3550308",
			"territory_name":   "São Paulo",
			"state_code":       "SP",
			"publication_urls": []string{"https://diario.sp.gov.br"},
			"level":            "3",
		},
		{
			"territory_id":     "3304557",
			"territory_name":   "Rio de Janeiro",
			"state_code":       "RJ",
			"publication_urls": []string{"https://diario.rj.gov.br"},
			"level":            "3",
		},
	},
}

var fakeThemesResponse = map[string]any{
	"themes": []string{
		"Políticas Ambientais",
		"Tecnologias na Educação",
	},
}

var fakeThemeSearchResponse = map[string]any{
	"total_excerpts": 42,
	"excerpts": []map[string]any{
		{
			"territory_id":   "5002704",
			"territory_name": "Campo Grande",
			"state_code":     "MS",
			"date":           "2023-05-16",
			"url":            "https://data.queridodiario.ok.org.br/5002704/2023-05-16/abc.pdf",
			"excerpt":        "compensação ambiental ...",
			"subthemes":      []string{"Política Nacional do Meio Ambiente"},
			"edition":        "7051",
		},
	},
}

// newQDRoutingServer creates a test server that routes by URL path, like the real QD API.
func newQDRoutingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case path == "/cities":
			json.NewEncoder(w).Encode(fakeCitiesResponse)
		case path == "/gazettes/by_theme/themes/" || path == "/gazettes/by_theme/themes":
			json.NewEncoder(w).Encode(fakeThemesResponse)
		case strings.HasPrefix(path, "/gazettes/by_theme/"):
			json.NewEncoder(w).Encode(fakeThemeSearchResponse)
		case path == "/gazettes":
			json.NewEncoder(w).Encode(fakeGazetteResponse)
		default:
			json.NewEncoder(w).Encode(fakeGazetteResponse)
		}
	}))
}

func newQDServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeGazetteResponse)
	}))
}

func TestQDCollector_Source(t *testing.T) {
	srv := newQDServer(t)
	defer srv.Close()
	c := dou.NewQDCollector(srv.URL)
	if c.Source() != "querido_diario" {
		t.Errorf("Source() = %q, want querido_diario", c.Source())
	}
}

func TestQDCollector_Search(t *testing.T) {
	srv := newQDServer(t)
	defer srv.Close()
	c := dou.NewQDCollector(srv.URL)
	records, err := c.Search(context.Background(), dou.SearchParams{Query: "contrato"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", records[0].Source)
	}
}

func TestQDCollector_ListCities(t *testing.T) {
	srv := newQDRoutingServer(t)
	defer srv.Close()
	c := dou.NewQDCollector(srv.URL)

	records, err := c.ListCities(context.Background())
	if err != nil {
		t.Fatalf("ListCities() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("ListCities() returned %d records, want 2", len(records))
	}
	if records[0].Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", records[0].Source)
	}
	if records[0].RecordKey != "3550308" {
		t.Errorf("RecordKey = %q, want 3550308", records[0].RecordKey)
	}
	if name, ok := records[0].Data["territory_name"].(string); !ok || name != "São Paulo" {
		t.Errorf("territory_name = %v, want São Paulo", records[0].Data["territory_name"])
	}
	if records[1].RecordKey != "3304557" {
		t.Errorf("RecordKey[1] = %q, want 3304557", records[1].RecordKey)
	}
}

func TestQDCollector_ListThemes(t *testing.T) {
	srv := newQDRoutingServer(t)
	defer srv.Close()
	c := dou.NewQDCollector(srv.URL)

	themes, err := c.ListThemes(context.Background())
	if err != nil {
		t.Fatalf("ListThemes() error: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("ListThemes() returned %d themes, want 2", len(themes))
	}
	if themes[0] != "Políticas Ambientais" {
		t.Errorf("themes[0] = %q, want Políticas Ambientais", themes[0])
	}
	if themes[1] != "Tecnologias na Educação" {
		t.Errorf("themes[1] = %q, want Tecnologias na Educação", themes[1])
	}
}

func TestQDCollector_SearchByTheme(t *testing.T) {
	srv := newQDRoutingServer(t)
	defer srv.Close()
	c := dou.NewQDCollector(srv.URL)

	records, err := c.SearchByTheme(context.Background(), "Políticas Ambientais", dou.SearchParams{Size: 2})
	if err != nil {
		t.Fatalf("SearchByTheme() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("SearchByTheme() returned %d records, want 1", len(records))
	}
	if records[0].Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", records[0].Source)
	}
	if records[0].Data["territory_name"] != "Campo Grande" {
		t.Errorf("territory_name = %v, want Campo Grande", records[0].Data["territory_name"])
	}
	if records[0].Data["state_code"] != "MS" {
		t.Errorf("state_code = %v, want MS", records[0].Data["state_code"])
	}
}
