# Novas Fontes BR-ACC — PGFN · PEP · Leniências · Renúncias · BNDES · TSE Filiados + Compliance Enrichment

> **For Claude:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Adicionar 6 endpoints novos e enriquecer `/v1/compliance/{cnpj}` com PGFN dívida ativa e acordos de leniência, seguindo o padrão de arquitetura existente.

**Architecture:** Endpoints do Portal da Transparência estendem `TransparenciaFederalHandler` com proxy direto (mesmo padrão de `GetCEAF`). `CGUCollector.FetchByCNPJ` ganha dois fetches soft-fail (PGFN + leniências) para enriquecer o compliance aggregate. BNDES ganha handler novo. TSE Filiados estende `TSEExtrasHandler` com o mesmo padrão ZIP/CSV já existente.

**Tech Stack:** Go 1.25 (GOTOOLCHAIN), Chi router, Portal da Transparência API (`TRANSPARENCIA_API_KEY`), BNDES Open Data CKAN, TSE CDN ZIP archives.

**Run tests with:** `~/.local/share/mise/installs/go/1.24.13/bin/go test ./...`

---

> ⚡ Tasks 1–4 são **INDEPENDENTES** — podem rodar em paralelo, cada uma toca arquivos distintos.
> Task 5 é **SEQUENCIAL** — só após tasks 1–4 completas e testes passando.

---

## Task 1: CGU Collector — enriquecer FetchByCNPJ com PGFN + Leniências

**Files:**
- Modify: `internal/collectors/transparencia/cgu.go`

**Context:**
`FetchByCNPJ` atualmente busca CEIS + CNEP e retorna um `SourceRecord` com chaves `ceis`, `cnep`, `sanitized`. O enriquecimento adiciona `pgfn` e `leniencias` com soft-fail (erro não bloqueia). O campo `sanitized` deve checar todos os 4 campos.

### Step 1: Write failing test

Adicione no final de `internal/collectors/transparencia/cgu.go` (ou crie `internal/collectors/transparencia/cgu_enrichment_test.go` se preferir separar):

```go
// No arquivo de testes existente ou novo:
func TestCGUCollector_FetchByCNPJ_EnrichedFields(t *testing.T) {
    // Servidor mock que responde a /ceis, /cnep, /pgfn, /leniencias
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch {
        case strings.Contains(r.URL.Path, "/ceis"):
            w.Write([]byte(`[]`))
        case strings.Contains(r.URL.Path, "/cnep"):
            w.Write([]byte(`[]`))
        case strings.Contains(r.URL.Path, "/pgfn"):
            w.Write([]byte(`[{"situacao":"REGULAR"}]`))
        case strings.Contains(r.URL.Path, "/leniencias"):
            w.Write([]byte(`[]`))
        default:
            http.NotFound(w, r)
        }
    }))
    defer srv.Close()

    c := NewCGUCollector(srv.URL, "test-key")
    records, err := c.FetchByCNPJ(context.Background(), "12345678000195")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 1 {
        t.Fatalf("expected 1 record, got %d", len(records))
    }
    data := records[0].Data
    if _, ok := data["pgfn"]; !ok {
        t.Error("expected 'pgfn' key in data")
    }
    if _, ok := data["leniencias"]; !ok {
        t.Error("expected 'leniencias' key in data")
    }
    // pgfn has 1 item, cnep/ceis empty → not sanitized
    if san, _ := data["sanitized"].(bool); san {
        t.Error("expected sanitized=false when pgfn has entries")
    }
}
```

### Step 2: Run — expect FAIL (fields missing)

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/collectors/transparencia/... -run TestCGUCollector_FetchByCNPJ_EnrichedFields -v
```

### Step 3: Implement — adicione helpers privados e atualize FetchByCNPJ

Em `internal/collectors/transparencia/cgu.go`, adicione após `fetchList`:

```go
// fetchPGFN busca dívida ativa no PGFN por CNPJ (soft-fail: retorna [] em caso de erro).
// Endpoint: GET /pgfn/consultaReceitaCadastro?cnpj={cnpj}&pagina=1
func (c *CGUCollector) fetchPGFN(ctx context.Context, cnpjNum string) []any {
    u := fmt.Sprintf("%s/pgfn/consultaReceitaCadastro?cnpj=%s&pagina=1", c.baseURL, cnpjNum)
    items, err := c.fetchURL(ctx, u)
    if err != nil {
        return []any{}
    }
    return items
}

// fetchLeniencias busca acordos de leniência por CNPJ (soft-fail).
// Endpoint: GET /leniencias?cnpj={cnpj}&pagina=1
func (c *CGUCollector) fetchLeniencias(ctx context.Context, cnpjNum string) []any {
    u := fmt.Sprintf("%s/leniencias?cnpj=%s&pagina=1", c.baseURL, cnpjNum)
    items, err := c.fetchURL(ctx, u)
    if err != nil {
        return []any{}
    }
    return items
}
```

Atualize `FetchByCNPJ` (substitua o bloco `record := ...`):

```go
func (c *CGUCollector) FetchByCNPJ(ctx context.Context, cnpjNum string) ([]domain.SourceRecord, error) {
    if c.apiKey == "" {
        return nil, fmt.Errorf("cgu_compliance: TRANSPARENCIA_API_KEY is not set")
    }

    ceis, err := c.fetchList(ctx, "/ceis", cnpjNum)
    if err != nil {
        return nil, fmt.Errorf("cgu_compliance: CEIS fetch: %w", err)
    }

    cnep, err := c.fetchList(ctx, "/cnep", cnpjNum)
    if err != nil {
        return nil, fmt.Errorf("cgu_compliance: CNEP fetch: %w", err)
    }

    // Soft-fail: PGFN e leniências enriquecem mas não bloqueiam
    pgfn := c.fetchPGFN(ctx, cnpjNum)
    leniencias := c.fetchLeniencias(ctx, cnpjNum)

    record := domain.SourceRecord{
        Source:    "cgu_compliance",
        RecordKey: cnpjNum,
        Data: map[string]any{
            "cnpj":       cnpjNum,
            "ceis":       ceis,
            "cnep":       cnep,
            "pgfn":       pgfn,
            "leniencias": leniencias,
            "sanitized":  len(ceis) == 0 && len(cnep) == 0 && len(pgfn) == 0 && len(leniencias) == 0,
        },
        FetchedAt: time.Now().UTC(),
    }

    return []domain.SourceRecord{record}, nil
}
```

### Step 4: Run — expect PASS

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/collectors/transparencia/... -v
```

### Step 5: Run full suite — expect no regressions

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./...
```

> **Note:** Os testes de `compliance_test.go` usam um stub `stubComplianceFetcher` com dados fixos — não são afetados por esta mudança. Os campos extras (`pgfn`, `leniencias`) vão aparecer na resposta real mas não quebram o handler.

### Step 6: Commit

```bash
git add internal/collectors/transparencia/cgu.go
git commit -m "feat: enrich FetchByCNPJ with PGFN and leniencias (soft-fail)"
```

---

## Task 2: TransparenciaFederalHandler — PGFN · PEP · Leniências · Renúncias

**Files:**
- Modify: `internal/handlers/transparencia_federal.go`
- Modify: `internal/handlers/transparencia_federal_test.go`

**Context:**
`TransparenciaFederalHandler` tem `h.httpClient` e `h.apiKey`. O padrão de `GetCEAF` (proxy direto sem usar o collector) é o modelo. Todos os 4 novos métodos seguem exatamente esse padrão. O helper `limitedReadAll` e `logUpstreamError` já existem em `helpers.go`.

### Step 1: Write failing tests

Adicione em `internal/handlers/transparencia_federal_test.go`:

```go
// ---- PGFN ----
func TestTransparenciaFederalHandler_GetPGFN_OK(t *testing.T) {
    apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, "pgfn") {
            w.Header().Set("Content-Type", "application/json")
            w.Write([]byte(`[{"situacao":"REGULAR","cnpj":"12345678000195"}]`))
            return
        }
        http.NotFound(w, r)
    }))
    defer apiSrv.Close()

    h := handlers.NewTransparenciaFederalHandlerWithClient(&stubFetcher{}, apiSrv.Client(), "test-key")
    // Note: precisamos de uma forma de substituir o base URL — ver implementação abaixo

    rtr := chi.NewRouter()
    rtr.Get("/v1/transparencia/pgfn/{cnpj}", h.GetPGFN)

    req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/pgfn/12345678000195", nil)
    req = x402pkg.InjectPrice(req, "0.003")
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }
}

func TestTransparenciaFederalHandler_GetPGFN_InvalidCNPJ(t *testing.T) {
    h := handlers.NewTransparenciaFederalHandlerWithClient(&stubFetcher{}, http.DefaultClient, "key")
    rtr := chi.NewRouter()
    rtr.Get("/v1/transparencia/pgfn/{cnpj}", h.GetPGFN)

    req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/pgfn/123", nil)
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}

// ---- PEP ----
func TestTransparenciaFederalHandler_GetPEP_OK(t *testing.T) {
    apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`[{"nome":"FULANO","cpf":"12345678901","cargo":"GOVERNADOR"}]`))
    }))
    defer apiSrv.Close()

    h := handlers.NewTransparenciaFederalHandlerWithClient(&stubFetcher{}, apiSrv.Client(), "test-key")
    rtr := chi.NewRouter()
    rtr.Get("/v1/transparencia/pep/{cpf}", h.GetPEP)

    req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/pep/12345678901", nil)
    req = x402pkg.InjectPrice(req, "0.003")
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }
    var resp domain.APIResponse
    json.NewDecoder(rec.Body).Decode(&resp)
    if resp.Source != "cgu_pep" {
        t.Errorf("Source = %q, want cgu_pep", resp.Source)
    }
}

func TestTransparenciaFederalHandler_GetPEP_InvalidCPF(t *testing.T) {
    h := handlers.NewTransparenciaFederalHandlerWithClient(&stubFetcher{}, http.DefaultClient, "key")
    rtr := chi.NewRouter()
    rtr.Get("/v1/transparencia/pep/{cpf}", h.GetPEP)

    req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/pep/123", nil)
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}

// ---- Leniências ----
func TestTransparenciaFederalHandler_GetLeniencias_OK(t *testing.T) {
    apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`[{"cnpj":"12345678000195","descricao":"Acordo de Leniencia"}]`))
    }))
    defer apiSrv.Close()

    h := handlers.NewTransparenciaFederalHandlerWithClient(&stubFetcher{}, apiSrv.Client(), "test-key")
    rtr := chi.NewRouter()
    rtr.Get("/v1/transparencia/leniencias/{cnpj}", h.GetLeniencias)

    req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/leniencias/12345678000195", nil)
    req = x402pkg.InjectPrice(req, "0.003")
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }
}

// ---- Renúncias ----
func TestTransparenciaFederalHandler_GetRenuncias_OK(t *testing.T) {
    apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`[{"beneficiario":"Empresa XYZ","valor":1000000}]`))
    }))
    defer apiSrv.Close()

    h := handlers.NewTransparenciaFederalHandlerWithClient(&stubFetcher{}, apiSrv.Client(), "test-key")
    rtr := chi.NewRouter()
    rtr.Get("/v1/transparencia/renuncias", h.GetRenuncias)

    req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/renuncias?ano=2024", nil)
    req = x402pkg.InjectPrice(req, "0.003")
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }
    var resp domain.APIResponse
    json.NewDecoder(rec.Body).Decode(&resp)
    if resp.Source != "cgu_renuncias" {
        t.Errorf("Source = %q, want cgu_renuncias", resp.Source)
    }
}
```

> **Note sobre os testes:** Os testes existentes do `transparencia_federal_test.go` usam `NewTransparenciaFederalHandlerWithClient(fetcher, client, key)`. Para os novos handlers que fazem proxy direto (sem passar pelo fetcher), o `h.httpClient` é o cliente injetado. Mas como o URL upstream está hardcoded em cada método, os testes precisam que o handler aceite um `baseURL` customizável. Veja a nota de implementação abaixo.

### Step 2: Run — expect FAIL

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/handlers/... -run "TestTransparenciaFederal.*PGFN\|TestTransparenciaFederal.*PEP\|TestTransparenciaFederal.*Lenien\|TestTransparenciaFederal.*Renun" -v
```

### Step 3: Implement

Em `internal/handlers/transparencia_federal.go`, adicione o campo `baseURL` à struct e atualize o construtor:

```go
type TransparenciaFederalHandler struct {
    fetcher    TransparenciaFetcher
    httpClient *http.Client
    apiKey     string
    baseURL    string // novo: base URL para proxy direto (default: transparenciaBase)
}

const transparenciaAPIBase = "https://api.portaldatransparencia.gov.br/api-de-dados"

func NewTransparenciaFederalHandler(f TransparenciaFetcher) *TransparenciaFederalHandler {
    apiKey := os.Getenv("TRANSPARENCIA_API_KEY")
    if apiKey == "" {
        slog.Warn("TRANSPARENCIA_API_KEY not set — transparencia endpoints will fail")
    }
    return &TransparenciaFederalHandler{
        fetcher:    f,
        httpClient: &http.Client{Timeout: 15 * time.Second},
        apiKey:     apiKey,
        baseURL:    transparenciaAPIBase,
    }
}

func NewTransparenciaFederalHandlerWithClient(f TransparenciaFetcher, client *http.Client, apiKey string) *TransparenciaFederalHandler {
    return &TransparenciaFederalHandler{
        fetcher:    f,
        httpClient: client,
        apiKey:     apiKey,
        baseURL:    transparenciaAPIBase,
    }
}

// NewTransparenciaFederalHandlerWithBaseURL cria um handler com base URL customizável (para testes).
func NewTransparenciaFederalHandlerWithBaseURL(f TransparenciaFetcher, client *http.Client, apiKey, baseURL string) *TransparenciaFederalHandler {
    return &TransparenciaFederalHandler{
        fetcher:    f,
        httpClient: client,
        apiKey:     apiKey,
        baseURL:    strings.TrimRight(baseURL, "/"),
    }
}
```

Adicione os 4 novos métodos ao final do arquivo:

```go
// GetPGFN handles GET /v1/transparencia/pgfn/{cnpj}
// Returns PGFN dívida ativa records for a given CNPJ.
// Requires TRANSPARENCIA_API_KEY.
func (h *TransparenciaFederalHandler) GetPGFN(w http.ResponseWriter, r *http.Request) {
    cnpj := normalizeCNPJdigits(chi.URLParam(r, "cnpj"))
    if len(cnpj) != 14 {
        jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
        return
    }

    upURL := fmt.Sprintf("%s/pgfn/consultaReceitaCadastro?cnpj=%s&pagina=1", h.baseURL, cnpj)
    req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
    if err != nil {
        internalError(w, "transparencia_pgfn", err)
        return
    }
    req.Header.Set("chave-api-dados", h.apiKey)

    resp, err := h.httpClient.Do(req)
    if err != nil {
        gatewayError(w, "transparencia_pgfn", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
        return
    }
    if resp.StatusCode == http.StatusNotFound {
        jsonError(w, http.StatusNotFound, "CNPJ não encontrado no PGFN: "+cnpj)
        return
    }
    if resp.StatusCode != http.StatusOK {
        body, _ := limitedReadAll(resp.Body)
        jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência PGFN", resp.StatusCode, body))
        return
    }

    var dados any
    if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
        gatewayError(w, "transparencia_pgfn", err)
        return
    }

    respond(w, r, domain.APIResponse{
        Source:   "cgu_pgfn",
        CostUSDC: x402pkg.PriceFromRequest(r),
        Data:     map[string]any{"cnpj": cnpj, "pgfn": dados},
    })
}

// GetPEP handles GET /v1/transparencia/pep/{cpf}
// Returns PEP (Pessoa Politicamente Exposta) records for a given CPF.
// Requires TRANSPARENCIA_API_KEY.
func (h *TransparenciaFederalHandler) GetPEP(w http.ResponseWriter, r *http.Request) {
    cpf := reDigits.ReplaceAllString(chi.URLParam(r, "cpf"), "")
    if len(cpf) != 11 {
        jsonError(w, http.StatusBadRequest, "CPF inválido — deve ter 11 dígitos")
        return
    }

    upURL := fmt.Sprintf("%s/pep?cpf=%s&pagina=1", h.baseURL, cpf)
    req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
    if err != nil {
        internalError(w, "transparencia_pep", err)
        return
    }
    req.Header.Set("chave-api-dados", h.apiKey)

    resp, err := h.httpClient.Do(req)
    if err != nil {
        gatewayError(w, "transparencia_pep", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
        return
    }
    if resp.StatusCode != http.StatusOK {
        body, _ := limitedReadAll(resp.Body)
        jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência PEP", resp.StatusCode, body))
        return
    }

    var dados any
    if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
        gatewayError(w, "transparencia_pep", err)
        return
    }

    respond(w, r, domain.APIResponse{
        Source:   "cgu_pep",
        CostUSDC: x402pkg.PriceFromRequest(r),
        Data:     map[string]any{"cpf": cpf, "pep": dados},
    })
}

// GetLeniencias handles GET /v1/transparencia/leniencias/{cnpj}
// Returns CGU leniency agreements for a given CNPJ.
// Requires TRANSPARENCIA_API_KEY.
func (h *TransparenciaFederalHandler) GetLeniencias(w http.ResponseWriter, r *http.Request) {
    cnpj := normalizeCNPJdigits(chi.URLParam(r, "cnpj"))
    if len(cnpj) != 14 {
        jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
        return
    }

    upURL := fmt.Sprintf("%s/leniencias?cnpj=%s&pagina=1", h.baseURL, cnpj)
    req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
    if err != nil {
        internalError(w, "transparencia_leniencias", err)
        return
    }
    req.Header.Set("chave-api-dados", h.apiKey)

    resp, err := h.httpClient.Do(req)
    if err != nil {
        gatewayError(w, "transparencia_leniencias", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
        return
    }
    if resp.StatusCode != http.StatusOK {
        body, _ := limitedReadAll(resp.Body)
        jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Leniências", resp.StatusCode, body))
        return
    }

    var dados any
    if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
        gatewayError(w, "transparencia_leniencias", err)
        return
    }

    respond(w, r, domain.APIResponse{
        Source:   "cgu_leniencias",
        CostUSDC: x402pkg.PriceFromRequest(r),
        Data:     map[string]any{"cnpj": cnpj, "leniencias": dados},
    })
}

// GetRenuncias handles GET /v1/transparencia/renuncias?ano=2024&n=20
// Returns fiscal tax waivers (renúncias fiscais) for a given year.
// Optional: ano (default current year), n (default 20, max 100).
// Requires TRANSPARENCIA_API_KEY.
func (h *TransparenciaFederalHandler) GetRenuncias(w http.ResponseWriter, r *http.Request) {
    ano := r.URL.Query().Get("ano")
    if ano == "" {
        ano = strconv.Itoa(time.Now().UTC().Year())
    }
    n := 20
    if raw := r.URL.Query().Get("n"); raw != "" {
        if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
            n = v
        }
    }

    upURL := fmt.Sprintf("%s/renuncias-fiscais?exercicio=%s&pagina=1&quantidade=%d", h.baseURL, ano, n)
    req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
    if err != nil {
        internalError(w, "transparencia_renuncias", err)
        return
    }
    req.Header.Set("chave-api-dados", h.apiKey)

    resp, err := h.httpClient.Do(req)
    if err != nil {
        gatewayError(w, "transparencia_renuncias", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
        return
    }
    if resp.StatusCode != http.StatusOK {
        body, _ := limitedReadAll(resp.Body)
        jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Renúncias", resp.StatusCode, body))
        return
    }

    var dados []any
    if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
        gatewayError(w, "transparencia_renuncias", err)
        return
    }

    respond(w, r, domain.APIResponse{
        Source:   "cgu_renuncias",
        CostUSDC: x402pkg.PriceFromRequest(r),
        Data:     map[string]any{"renuncias": dados, "total": len(dados), "ano": ano},
    })
}
```

**Atualize os testes** para usar `NewTransparenciaFederalHandlerWithBaseURL(fetcher, srv.Client(), "test-key", srv.URL)` onde o servidor mock está rodando.

### Step 4: Run tests

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/handlers/... -run "TestTransparenciaFederal" -v
```

### Step 5: Run full suite

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./...
```

### Step 6: Commit

```bash
git add internal/handlers/transparencia_federal.go internal/handlers/transparencia_federal_test.go
git commit -m "feat: add PGFN, PEP, leniencias, renuncias endpoints to TransparenciaFederalHandler"
```

---

## Task 3: BNDES Handler (novo arquivo)

**Files:**
- Create: `internal/handlers/bndes.go`
- Create: `internal/handlers/bndes_test.go`

**Context:**
BNDES disponibiliza dados no portal CKAN: `https://dadosabertos.bndes.gov.br`.
O endpoint usa uma busca SQL no datastore. A lógica é:
1. Fazer `GET /api/3/action/package_show?id=operacoes-de-credito-direto-e-indireto` para obter o `resource_id` mais recente
2. Fazer `GET /api/3/action/datastore_search?resource_id={id}&q={cnpj}&limit={n}` para buscar operações

> ⚠️ **Verificar durante implementação:** O `id` do pacote pode ser `operacoes-de-credito-direto-e-indireto` ou outro nome. Verifique em `https://dadosabertos.bndes.gov.br/dataset` antes de implementar.
> Se o CKAN não funcionar, fallback: `GET https://www.bndes.gov.br/bndesop/pesquisa/buscarOperacoes.json?filtros[cnpj]={cnpj}&tipoPesquisa=2` (API interna do site BNDES, sem autenticação).

### Step 1: Write failing test

Crie `internal/handlers/bndes_test.go`:

```go
package handlers_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/databr/api/internal/domain"
    "github.com/databr/api/internal/handlers"
    x402pkg "github.com/databr/api/internal/x402"
    "github.com/go-chi/chi/v5"
)

func TestBNDESHandler_GetOperacoes_OK(t *testing.T) {
    // Mock CKAN server
    bndesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        if strings.Contains(r.URL.Path, "package_show") {
            // Retorna um resource_id de teste
            json.NewEncoder(w).Encode(map[string]any{
                "success": true,
                "result": map[string]any{
                    "resources": []map[string]any{
                        {"id": "fake-resource-id", "format": "CSV"},
                    },
                },
            })
            return
        }
        if strings.Contains(r.URL.Path, "datastore_search") {
            json.NewEncoder(w).Encode(map[string]any{
                "success": true,
                "result": map[string]any{
                    "records": []map[string]any{
                        {"cnpj_empresa": "12345678000195", "valor_contratado": 1000000, "produto": "FINANCIAMENTO"},
                    },
                    "total": 1,
                },
            })
            return
        }
        http.NotFound(w, r)
    }))
    defer bndesSrv.Close()

    h := handlers.NewBNDESHandlerWithBaseURL(bndesSrv.URL)
    rtr := chi.NewRouter()
    rtr.Get("/v1/bndes/operacoes/{cnpj}", h.GetOperacoes)

    req := httptest.NewRequest(http.MethodGet, "/v1/bndes/operacoes/12345678000195", nil)
    req = x402pkg.InjectPrice(req, "0.005")
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }

    var resp domain.APIResponse
    if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if resp.Source != "bndes_operacoes" {
        t.Errorf("Source = %q, want bndes_operacoes", resp.Source)
    }
    if resp.CostUSDC != "0.005" {
        t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
    }
}

func TestBNDESHandler_GetOperacoes_InvalidCNPJ(t *testing.T) {
    h := handlers.NewBNDESHandlerWithBaseURL("http://unused")
    rtr := chi.NewRouter()
    rtr.Get("/v1/bndes/operacoes/{cnpj}", h.GetOperacoes)

    req := httptest.NewRequest(http.MethodGet, "/v1/bndes/operacoes/123", nil)
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}
```

### Step 2: Run — expect FAIL (package not found)

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/handlers/... -run "TestBNDES" -v
```

### Step 3: Implement

Crie `internal/handlers/bndes.go`:

```go
package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "regexp"
    "time"

    "github.com/databr/api/internal/domain"
    x402pkg "github.com/databr/api/internal/x402"
    "github.com/go-chi/chi/v5"
)

const bndesCKANBase = "https://dadosabertos.bndes.gov.br"
const bndespPkgID = "operacoes-de-credito-direto-e-indireto"

// BNDESHandler handles on-demand BNDES open data requests.
type BNDESHandler struct {
    httpClient *http.Client
    baseURL    string
}

// NewBNDESHandler creates a BNDESHandler using the BNDES CKAN open data API.
func NewBNDESHandler() *BNDESHandler {
    return &BNDESHandler{
        httpClient: &http.Client{Timeout: 20 * time.Second},
        baseURL:    bndesCKANBase,
    }
}

// NewBNDESHandlerWithBaseURL creates a BNDESHandler with a custom base URL (for testing).
func NewBNDESHandlerWithBaseURL(baseURL string) *BNDESHandler {
    return &BNDESHandler{
        httpClient: &http.Client{Timeout: 20 * time.Second},
        baseURL:    baseURL,
    }
}

var reDigitsBNDES = regexp.MustCompile(`\D`)

// GetOperacoes handles GET /v1/bndes/operacoes/{cnpj}?n=20
// Returns BNDES financing operations for a given CNPJ.
// Uses the BNDES CKAN open data portal (no API key required).
func (h *BNDESHandler) GetOperacoes(w http.ResponseWriter, r *http.Request) {
    cnpj := reDigitsBNDES.ReplaceAllString(chi.URLParam(r, "cnpj"), "")
    if len(cnpj) != 14 {
        jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
        return
    }
    n := 20
    if raw := r.URL.Query().Get("n"); raw != "" {
        var v int
        if _, err := fmt.Sscanf(raw, "%d", &v); err == nil && v > 0 && v <= 100 {
            n = v
        }
    }

    // Step 1: get current resource_id from CKAN package
    pkgURL := fmt.Sprintf("%s/api/3/action/package_show?id=%s", h.baseURL, bndespPkgID)
    pkgReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, pkgURL, nil)
    if err != nil {
        internalError(w, "bndes", err)
        return
    }
    pkgResp, err := h.httpClient.Do(pkgReq)
    if err != nil {
        gatewayError(w, "bndes", err)
        return
    }
    defer pkgResp.Body.Close()

    var pkgResult struct {
        Success bool `json:"success"`
        Result  struct {
            Resources []struct {
                ID     string `json:"id"`
                Format string `json:"format"`
            } `json:"resources"`
        } `json:"result"`
    }
    if err := json.NewDecoder(pkgResp.Body).Decode(&pkgResult); err != nil || !pkgResult.Success || len(pkgResult.Result.Resources) == 0 {
        jsonError(w, http.StatusBadGateway, "BNDES: não foi possível obter resource_id do dataset")
        return
    }
    resourceID := pkgResult.Result.Resources[0].ID

    // Step 2: search datastore by CNPJ
    searchURL := fmt.Sprintf(
        "%s/api/3/action/datastore_search?resource_id=%s&q=%s&limit=%d",
        h.baseURL, resourceID, cnpj, n,
    )
    searchReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, searchURL, nil)
    if err != nil {
        internalError(w, "bndes", err)
        return
    }
    searchResp, err := h.httpClient.Do(searchReq)
    if err != nil {
        gatewayError(w, "bndes", err)
        return
    }
    defer searchResp.Body.Close()

    var searchResult struct {
        Success bool `json:"success"`
        Result  struct {
            Records []any  `json:"records"`
            Total   int    `json:"total"`
        } `json:"result"`
    }
    if err := json.NewDecoder(searchResp.Body).Decode(&searchResult); err != nil || !searchResult.Success {
        jsonError(w, http.StatusBadGateway, "BNDES: erro ao buscar operações")
        return
    }

    if len(searchResult.Result.Records) == 0 {
        jsonError(w, http.StatusNotFound, "Nenhuma operação BNDES encontrada para CNPJ "+cnpj)
        return
    }

    respond(w, r, domain.APIResponse{
        Source:    "bndes_operacoes",
        UpdatedAt: time.Now().UTC(),
        CostUSDC:  x402pkg.PriceFromRequest(r),
        Data: map[string]any{
            "cnpj":      cnpj,
            "operacoes": searchResult.Result.Records,
            "total":     searchResult.Result.Total,
        },
    })
}
```

> **Note:** `reDigitsBNDES` é definido localmente para não colidir com `reDigits` de `helpers.go`. Alternativamente, use `reDigits` se for visível no mesmo package — verifique em `helpers.go`.

### Step 4: Run tests

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/handlers/... -run "TestBNDES" -v
```

### Step 5: Run full suite

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./...
```

### Step 6: Commit

```bash
git add internal/handlers/bndes.go internal/handlers/bndes_test.go
git commit -m "feat: add BNDESHandler for BNDES open data operations by CNPJ"
```

---

## Task 4: TSE Filiados

**Files:**
- Modify: `internal/handlers/tse_extras.go`
- Modify: `internal/handlers/tse_extras_test.go`

**Context:**
TSE disponibiliza dados de filiados partidários como ZIPs com CSVs em Latin-1. O `TSEExtrasHandler` já tem `downloadZip` e `parseZipCSV`. As funções `parseLimitN` e `latestElectionYear` também já existem.

URL base para filiados (diferente do `odsele`):
`https://cdn.tse.jus.br/estatistica/sead/eleitorado/filiados/uf/filiados_{uf_lower}.zip`

> ⚠️ **Verificar durante implementação:** Confira o padrão exato em `https://dadosabertos.tse.jus.br/dataset/filiados-partidos`. O caminho pode incluir um timestamp/data. Se o arquivo for por partido+UF, o URL pode ser `filiados_{sigla_lower}_{uf_lower}.zip`. Se o download falhar, tente o CKAN: `https://dadosabertos.tse.jus.br/api/3/action/package_show?id=filiados-partidos`.

### Step 1: Write failing test

Adicione em `internal/handlers/tse_extras_test.go`:

```go
func TestTSEExtrasHandler_GetFiliados_OK(t *testing.T) {
    // Cria um ZIP fake com CSV de filiados
    zipData := makeFakeCSVZip("filiados_sp.csv",
        "sg_partido;nm_partido;nm_municipio;sg_uf;dt_filiacao;nm_filiado\n"+
        "PT;PARTIDO DOS TRABALHADORES;SAO PAULO;SP;2020-01-15;FULANO DE TAL\n",
    )

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/zip")
        w.Write(zipData)
    }))
    defer srv.Close()

    h := handlers.NewTSEExtrasHandlerWithClient(srv.Client(), srv.URL)
    rtr := chi.NewRouter()
    rtr.Get("/v1/eleicoes/filiados", h.GetFiliados)

    req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/filiados?uf=SP&n=10", nil)
    req = x402pkg.InjectPrice(req, "0.003")
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }

    var resp domain.APIResponse
    if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if resp.Source != "tse_filiados" {
        t.Errorf("Source = %q, want tse_filiados", resp.Source)
    }
}

func TestTSEExtrasHandler_GetFiliados_MissingUF(t *testing.T) {
    h := handlers.NewTSEExtrasHandlerWithClient(http.DefaultClient, "http://unused")
    rtr := chi.NewRouter()
    rtr.Get("/v1/eleicoes/filiados", h.GetFiliados)

    req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/filiados", nil) // sem ?uf=
    rec := httptest.NewRecorder()
    rtr.ServeHTTP(rec, req)

    if rec.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}
```

> **Note sobre `makeFakeCSVZip`:** Esta helper pode já existir nos testes. Se não existir, veja como `tse_extras_test.go` cria ZIPs de teste para os outros métodos e replique o padrão. Caso não haja tal helper, crie em `helpers_test.go` ou inline no teste.

### Step 2: Run — expect FAIL

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/handlers/... -run "TestTSEExtras.*Filiados" -v
```

### Step 3: Implement

Adicione ao final de `internal/handlers/tse_extras.go`:

```go
const tseFiliadosBase = "https://cdn.tse.jus.br/estatistica/sead/eleitorado/filiados/uf"

// GetFiliados handles GET /v1/eleicoes/filiados?uf=SP&n=100
// Downloads TSE party membership (filiados) ZIP for a given state (UF).
// Required: uf (2-letter state code, e.g. "SP").
// Optional: n (default 100, max 500).
func (h *TSEExtrasHandler) GetFiliados(w http.ResponseWriter, r *http.Request) {
    uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
    if len(uf) != 2 {
        jsonError(w, http.StatusBadRequest, "query param 'uf' é obrigatório (ex: SP, RJ, MG)")
        return
    }
    n := parseLimitN(r, 100, 500)

    // URL pattern: filiados_{uf_lower}.zip
    // Verify at https://dadosabertos.tse.jus.br/dataset/filiados-partidos if this changes.
    zipURL := fmt.Sprintf("%s/filiados_%s.zip", tseFiliadosBase, strings.ToLower(uf))

    zipData, err := h.downloadZip(r, zipURL)
    if err != nil {
        gatewayError(w, "tse_filiados", err)
        return
    }

    rows, err := parseZipCSV(zipData, n)
    if err != nil {
        gatewayError(w, "tse_filiados", err)
        return
    }

    if len(rows) == 0 {
        jsonError(w, http.StatusNotFound, "tse_filiados: nenhum registro encontrado para UF "+uf)
        return
    }

    respond(w, r, domain.APIResponse{
        Source:    "tse_filiados",
        UpdatedAt: time.Now().UTC(),
        CostUSDC:  x402pkg.PriceFromRequest(r),
        Data:      map[string]any{"filiados": rows, "total": len(rows), "uf": uf},
    })
}
```

> **Note:** A constante `tseFiliadosBase` usa um caminho diferente de `h.baseURL` (que aponta para `odsele`). Por isso é hardcoded como constante. O handler usa `h.downloadZip` que já aceita uma URL arbitrária.

### Step 4: Run tests

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/handlers/... -run "TestTSEExtras" -v
```

### Step 5: Run full suite

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./...
```

### Step 6: Commit

```bash
git add internal/handlers/tse_extras.go internal/handlers/tse_extras_test.go
git commit -m "feat: add TSE filiados endpoint (party membership by UF)"
```

---

## Task 5: Pricing · Routing · MCP (SEQUENCIAL — após Tasks 1–4)

**Files:**
- Modify: `internal/x402/pricing.go`
- Modify: `cmd/api/main.go`
- Modify: `internal/mcp/server.go`

**Prerequisites:** Tasks 1–4 completas e `go test ./...` passando.

### Step 1: Pricing — adicione as novas rotas

Em `internal/x402/pricing.go`, adicione ao `priceTable`:

```go
// Novas fontes (br-acc phase)
"/v1/transparencia/pgfn/{cnpj}":      "0.003",
"/v1/transparencia/pep/{cpf}":        "0.003",
"/v1/transparencia/leniencias/{cnpj}":"0.003",
"/v1/transparencia/renuncias":         "0.003",
"/v1/bndes/operacoes/{cnpj}":         "0.005",
"/v1/eleicoes/filiados":              "0.003",
```

### Step 2: Run pricing tests

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go test ./internal/x402/... -v
```

### Step 3: Routing — registre os endpoints em main.go

Em `cmd/api/main.go`:

**Declaração do handler** (junto aos outros on-demand handlers, ~linha 125):
```go
bndesHandler := handlers.NewBNDESHandler()
```

**No grupo `$0.003`** (junto com `/transparencia/ceaf/{cnpj}`, ~linha 704):
```go
r.Get("/transparencia/pgfn/{cnpj}", transparenciaFedHandler.GetPGFN)
r.Get("/transparencia/pep/{cpf}", transparenciaFedHandler.GetPEP)
r.Get("/transparencia/leniencias/{cnpj}", transparenciaFedHandler.GetLeniencias)
r.Get("/transparencia/renuncias", transparenciaFedHandler.GetRenuncias)
r.Get("/eleicoes/filiados", tseExtrasHandler.GetFiliados)
```

**No grupo `$0.005`** (junto com `/mercado/acoes/{ticker}`, ~linha 785):
```go
r.Get("/bndes/operacoes/{cnpj}", bndesHandler.GetOperacoes)
```

### Step 4: MCP — adicione 3 ferramentas de alto valor

Em `internal/mcp/server.go`, adicione na struct `HandlerDeps`:
```go
TranspPGFN       http.HandlerFunc // GET /v1/transparencia/pgfn/{cnpj}
TranspPEP        http.HandlerFunc // GET /v1/transparencia/pep/{cpf}
BNDESOperacoes   http.HandlerFunc // GET /v1/bndes/operacoes/{cnpj}
TSEFiliados      http.HandlerFunc // GET /v1/eleicoes/filiados
```

Em `cmd/api/main.go`, no bloco `mcpDeps` (junto com TranspViagens, ~linha 239):
```go
TranspPGFN:       transparenciaFedHandler.GetPGFN,
TranspPEP:        transparenciaFedHandler.GetPEP,
BNDESOperacoes:   bndesHandler.GetOperacoes,
TSEFiliados:      tseExtrasHandler.GetFiliados,
```

Em `internal/mcp/server.go`, registre as ferramentas MCP no método `NewServer` usando o padrão existente (veja como `TranspViagens` e `TSEBens` são registrados como modelo):

```go
// Consulta PGFN — dívida ativa da União por CNPJ
s.AddTool(mcpgosdk.NewTool("consultar_pgfn",
    mcpgosdk.WithDescription("Consulta dívida ativa da União no PGFN para um CNPJ"),
    mcpgosdk.WithString("cnpj", mcpgosdk.Required(), mcpgosdk.Description("CNPJ (14 dígitos, com ou sem formatação)")),
), makeChiHandler(deps.TranspPGFN, "/v1/transparencia/pgfn/{cnpj}", "cnpj"))

// Consulta PEP — pessoa politicamente exposta por CPF
s.AddTool(mcpgosdk.NewTool("consultar_pep",
    mcpgosdk.WithDescription("Consulta se um CPF consta na lista de Pessoas Politicamente Expostas (PEP) do CGU"),
    mcpgosdk.WithString("cpf", mcpgosdk.Required(), mcpgosdk.Description("CPF (11 dígitos, com ou sem formatação)")),
), makeChiHandler(deps.TranspPEP, "/v1/transparencia/pep/{cpf}", "cpf"))

// Consulta BNDES — operações de financiamento por CNPJ
s.AddTool(mcpgosdk.NewTool("consultar_bndes",
    mcpgosdk.WithDescription("Consulta operações de crédito/financiamento do BNDES para um CNPJ"),
    mcpgosdk.WithString("cnpj", mcpgosdk.Required(), mcpgosdk.Description("CNPJ (14 dígitos, com ou sem formatação)")),
    mcpgosdk.WithNumber("n", mcpgosdk.Description("Número máximo de registros (padrão 20, max 100)")),
), makeQueryHandler(deps.BNDESOperacoes, map[string]string{}))

// Consulta TSE Filiados — filiação partidária por UF
s.AddTool(mcpgosdk.NewTool("consultar_filiados",
    mcpgosdk.WithDescription("Consulta filiados partidários por UF a partir dos dados do TSE"),
    mcpgosdk.WithString("uf", mcpgosdk.Required(), mcpgosdk.Description("UF (2 letras, ex: SP, RJ)")),
    mcpgosdk.WithNumber("n", mcpgosdk.Description("Número máximo de registros (padrão 100, max 500)")),
), makeQueryHandler(deps.TSEFiliados, map[string]string{}))
```

> **Note sobre MCP helpers:** Verifique como `makeChiHandler` e `makeQueryHandler` (ou equivalentes) são definidos em `server.go` — o padrão exato pode variar. Use o mesmo padrão dos tools existentes como `consultar_pep` ou `TranspViagens`.

### Step 5: Build e testes completos

```bash
~/.local/share/mise/installs/go/1.24.13/bin/go build ./...
~/.local/share/mise/installs/go/1.24.13/bin/go test ./...
```

### Step 6: Commit final

```bash
git add internal/x402/pricing.go cmd/api/main.go internal/mcp/server.go
git commit -m "feat: wire PGFN, PEP, leniencias, renuncias, BNDES, filiados into pricing/routing/MCP"
```

---

## Checklist Final

- [ ] Task 1: CGU FetchByCNPJ enriquecida (pgfn + leniencias soft-fail)
- [ ] Task 2: 4 handlers novos no TransparenciaFederalHandler
- [ ] Task 3: BNDESHandler criado e testado
- [ ] Task 4: TSE Filiados no TSEExtrasHandler
- [ ] Task 5: pricing.go, main.go, mcp/server.go atualizados
- [ ] `go test ./...` passa sem erros
- [ ] `go build ./...` sem erros

## Notas de Verificação de URLs (runtime)

| Fonte | URL a verificar |
|-------|----------------|
| PGFN | `https://api.portaldatransparencia.gov.br/swagger-ui.html` → pesquisar "pgfn" |
| PEP | `https://api.portaldatransparencia.gov.br/swagger-ui.html` → pesquisar "pep" |
| Leniências | `https://api.portaldatransparencia.gov.br/swagger-ui.html` → pesquisar "leniencias" |
| Renúncias | `https://api.portaldatransparencia.gov.br/swagger-ui.html` → pesquisar "renuncias" |
| BNDES | `https://dadosabertos.bndes.gov.br/dataset/operacoes-de-credito-direto-e-indireto` |
| TSE Filiados | `https://dadosabertos.tse.jus.br/dataset/filiados-partidos` |

Se algum endpoint retornar 404 real (não mockado), ajuste o URL com base na documentação oficial antes de fazer commit.
