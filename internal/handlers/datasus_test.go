package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDATASUSHandler_GetEstabelecimento(t *testing.T) {
	// Mock upstream DATASUS API
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cnes/estabelecimentos/9629866" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"codigo_cnes":                    9629866,
			"nome_razao_social":              "FERNANDO NUNES AGUIAR",
			"nome_fantasia":                  "FERNANDO NUNES AGUIAR",
			"descricao_esfera_administrativa": "MUNICIPAL",
			"codigo_municipio":               330240,
		})
	}))
	defer upstream.Close()

	h := &DATASUSHandler{
		baseURL: upstream.URL,
		client:  upstream.Client(),
	}

	r := chi.NewRouter()
	r.Get("/v1/saude/estabelecimentos/{cnes}", h.GetEstabelecimento)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/estabelecimentos/9629866", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["source"] != "datasus_cnes" {
		t.Errorf("expected source datasus_cnes, got %v", resp["source"])
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp["data"])
	}
	if data["nome_fantasia"] != "FERNANDO NUNES AGUIAR" {
		t.Errorf("unexpected nome_fantasia: %v", data["nome_fantasia"])
	}
}

func TestDATASUSHandler_GetEstabelecimentos(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cnes/estabelecimentos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		if q.Get("codigo_municipio") != "330455" {
			t.Errorf("expected codigo_municipio=330455, got %s", q.Get("codigo_municipio"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"estabelecimentos": []map[string]any{
				{"codigo_cnes": 1082493, "nome_fantasia": "HOSPITAL A"},
				{"codigo_cnes": 1082494, "nome_fantasia": "HOSPITAL B"},
			},
		})
	}))
	defer upstream.Close()

	h := &DATASUSHandler{
		baseURL: upstream.URL,
		client:  upstream.Client(),
	}

	r := chi.NewRouter()
	r.Get("/v1/saude/estabelecimentos", h.GetEstabelecimentos)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/estabelecimentos?municipio=330455", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", resp["data"])
	}
	total, ok := data["total"].(float64)
	if !ok || total != 2 {
		t.Errorf("expected total=2, got %v", data["total"])
	}
}

func TestDATASUSHandler_GetEstabelecimentos_RequiresFilter(t *testing.T) {
	h := NewDATASUSHandler()

	r := chi.NewRouter()
	r.Get("/v1/saude/estabelecimentos", h.GetEstabelecimentos)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/estabelecimentos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
