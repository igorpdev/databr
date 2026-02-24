package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	x402pkg "github.com/databr/api/internal/x402"
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

func TestDATASUSHandler_GetMortalidade(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vigilancia-e-meio-ambiente/sistema-de-informacao-sobre-mortalidade" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		if q.Get("limit") != "50" || q.Get("offset") != "0" {
			t.Errorf("unexpected query: limit=%s offset=%s", q.Get("limit"), q.Get("offset"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"sim": []map[string]any{
				{"causabas": "I219", "sexo": "1", "idade": "475", "codmunres": "421010"},
				{"causabas": "P015", "sexo": "1", "idade": "113", "codmunres": "330455"},
			},
		})
	}))
	defer upstream.Close()

	h := &DATASUSHandler{baseURL: upstream.URL, client: upstream.Client()}
	r := chi.NewRouter()
	r.Get("/v1/saude/mortalidade", h.GetMortalidade)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/mortalidade", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["source"] != "datasus_sim" {
		t.Errorf("expected source datasus_sim, got %v", resp["source"])
	}
	if resp["cost_usdc"] != "0.005" {
		t.Errorf("expected cost_usdc 0.005, got %v", resp["cost_usdc"])
	}
	data := resp["data"].(map[string]any)
	if data["total"].(float64) != 2 {
		t.Errorf("expected total=2, got %v", data["total"])
	}
}

func TestDATASUSHandler_GetNascimentos(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vigilancia-e-meio-ambiente/sistema-de-informacao-sobre-nascidos-vivos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"sinasc": []map[string]any{
				{"peso": "3050", "sexo": "2", "codmunnasc": "431640"},
			},
		})
	}))
	defer upstream.Close()

	h := &DATASUSHandler{baseURL: upstream.URL, client: upstream.Client()}
	r := chi.NewRouter()
	r.Get("/v1/saude/nascimentos", h.GetNascimentos)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/nascimentos?limit=10&offset=5", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["source"] != "datasus_sinasc" {
		t.Errorf("expected source datasus_sinasc, got %v", resp["source"])
	}
	data := resp["data"].(map[string]any)
	if data["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", data["total"])
	}
}

func TestDATASUSHandler_GetHospitais(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/assistencia-a-saude/hospitais-e-leitos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"hospitais_leitos": []map[string]any{
				{"nome_do_hospital": "HOSPITAL DO RIM", "quantidade_total_de_leitos_do_hosptial": 29.0},
				{"nome_do_hospital": "HOSPITAL ROQUE GONZALES", "quantidade_total_de_leitos_do_hosptial": 41.0},
			},
		})
	}))
	defer upstream.Close()

	h := &DATASUSHandler{baseURL: upstream.URL, client: upstream.Client()}
	r := chi.NewRouter()
	r.Get("/v1/saude/hospitais", h.GetHospitais)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/hospitais", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["source"] != "datasus_hospitais" {
		t.Errorf("expected source datasus_hospitais, got %v", resp["source"])
	}
	data := resp["data"].(map[string]any)
	if data["total"].(float64) != 2 {
		t.Errorf("expected total=2, got %v", data["total"])
	}
}

func TestDATASUSHandler_GetDengue(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/arboviroses/dengue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"parametros": []map[string]any{
				{"id_agravo": "A90", "dt_notific": "2021-01-30", "cs_sexo": "M"},
			},
		})
	}))
	defer upstream.Close()

	h := &DATASUSHandler{baseURL: upstream.URL, client: upstream.Client()}
	r := chi.NewRouter()
	r.Get("/v1/saude/dengue", h.GetDengue)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/dengue?limit=5", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["source"] != "datasus_dengue" {
		t.Errorf("expected source datasus_dengue, got %v", resp["source"])
	}
	data := resp["data"].(map[string]any)
	if data["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", data["total"])
	}
}

func TestDATASUSHandler_GetVacinacao(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vacinacao/doses-aplicadas-pni-2025" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"doses_aplicadas_pni": []map[string]any{
				{"sigla_vacina": "PENTA", "descricao_dose_vacina": "3a Dose", "tipo_sexo_paciente": "M"},
			},
		})
	}))
	defer upstream.Close()

	h := &DATASUSHandler{baseURL: upstream.URL, client: upstream.Client()}
	r := chi.NewRouter()
	r.Get("/v1/saude/vacinacao/{ano}", h.GetVacinacao)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/vacinacao/2025", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["source"] != "datasus_vacinacao" {
		t.Errorf("expected source datasus_vacinacao, got %v", resp["source"])
	}
	data := resp["data"].(map[string]any)
	if data["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", data["total"])
	}
}

func TestDATASUSHandler_GetVacinacao_InvalidYear(t *testing.T) {
	h := NewDATASUSHandler()

	r := chi.NewRouter()
	r.Get("/v1/saude/vacinacao/{ano}", h.GetVacinacao)

	tests := []struct {
		name string
		year string
	}{
		{"too_old", "2019"},
		{"too_new", "2031"},
		{"not_a_number", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/saude/vacinacao/"+tt.year, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for year %s, got %d: %s", tt.year, w.Code, w.Body.String())
			}
		})
	}
}
