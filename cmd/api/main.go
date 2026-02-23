package main

import (
	"context"
	"crypto/tls"
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cnpjcol "github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/collectors/dou"
	"github.com/databr/api/internal/collectors/juridico"
	"github.com/databr/api/internal/collectors/tesouro"
	"github.com/databr/api/internal/collectors/transparencia"
	"github.com/databr/api/internal/cache"
	docfs "github.com/databr/api/docs"
	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/databr/api/internal/logging"
	"github.com/databr/api/internal/mcp"
	"github.com/databr/api/internal/repositories"
	x402pkg "github.com/databr/api/internal/x402"
	migpkg "github.com/databr/api/migrations"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/joho/godotenv"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	logging.Setup(nil)

	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file, using environment variables")
	}

	startTime := time.Now()
	ctx := context.Background()

	// Database (optional — store-backed handlers degrade gracefully when nil)
	var store handlers.SourceStore
	var pool interface{ Ping(context.Context) error }
	if p, err := repositories.NewPool(ctx); err != nil {
		slog.Warn("database unavailable, store-backed endpoints disabled", "error", err)
	} else {
		if err := repositories.RunMigrations(ctx, p, migpkg.FS); err != nil {
			slog.Warn("migrations failed", "error", err)
		}
		store = repositories.NewSourceRecordRepository(p)
		pool = p
		defer p.Close()
	}

	// Redis cache (optional — degrades gracefully when unavailable)
	var cacher cache.Cacher
	if rc, err := cache.NewClient(); err != nil {
		slog.Warn("Redis unavailable, cache disabled", "error", err)
	} else {
		cacher = rc
		defer rc.Close()
	}

	// x402 payment config (wallet required; set to empty string in dev = no-op middleware)
	x402Cfg := x402pkg.MiddlewareConfig{
		WalletAddress:  os.Getenv("WALLET_ADDRESS"),
		FacilitatorURL: os.Getenv("X402_FACILITATOR_URL"),
		Network:        networkName(os.Getenv("X402_NETWORK")),
		CDPKeyID:       os.Getenv("CDP_KEY_ID"),
		CDPKeySecret:   os.Getenv("CDP_KEY_SECRET"),
	}
	if x402Cfg.FacilitatorURL == "" {
		x402Cfg.FacilitatorURL = "https://x402.org/facilitator"
	}

	// On-demand collectors (no DB write; data returned directly to HTTP handler)
	cnpjCollector := cnpjcol.NewCollector("")
	cguCollector := transparencia.NewCGUCollector("", os.Getenv("TRANSPARENCIA_API_KEY"))

	// On-demand collectors for new endpoints
	tesouroCol := tesouro.NewSICONFICollector("")
	qdCollector := dou.NewQDCollector("")
	djCollector := juridico.NewDataJudCollector("", os.Getenv("DATAJUD_API_KEY"))

	// HTTP handlers (on-demand, always available)
	empHandler := handlers.NewEmpresasHandler(cnpjCollector)
	compHandler := handlers.NewComplianceHandler(cguCollector)
	enderecoHandler := handlers.NewEnderecoHandler()
	transparenciaFedHandler := handlers.NewTransparenciaFederalHandler(cguCollector)
	tesouroHand := handlers.NewTesouroHandler(tesouroCol)
	douHandler := handlers.NewDOUHandler(qdCollector)
	judicialHand := handlers.NewJudicialHandler(djCollector)
	ibgeHandler := handlers.NewIbgeHandler()
	legislativoHandler := handlers.NewLegislativoHandler()
	ipeaHandler := handlers.NewIPEAHandler()
	pncpHandler := handlers.NewPNCPHandler()
	tseExtrasHandler := handlers.NewTSEExtrasHandler()
	ansHandler := handlers.NewANSHandler()
	tcuHandler := handlers.NewTCUHandler()
	orcamentoHandler := handlers.NewOrcamentoHandler()
	// Proxy BCB handler for routes that call external APIs directly (no DB needed).
	proxyBCBHandler := handlers.NewBCBHandler(nil)

	// Store-backed handlers (only available when DB is connected)
	var (
		bcbHandler             *handlers.BCBHandler
		ecoHandler             *handlers.EconomiaHandler
		mercHandler            *handlers.MercadoHandler
		transHandler           *handlers.TransparenciaHandler
		saudeHandler           *handlers.SaudeHandler
		energiaHandler         *handlers.EnergiaHandler
		ambientalHandler       *handlers.AmbientalHandler
		transporteHandler      *handlers.TransporteHandler
		transportadoresHandler *handlers.TransportadoresHandler
		titulosHandler         *handlers.TitulosHandler
		comercioHandler        *handlers.ComercioHandler
		educacaoHandler        *handlers.EducacaoHandler
		empregoHandler         *handlers.EmpregoHandler
		// Premium cross-referencing handlers (Phase 10)
		dueDiligenceHandler        *handlers.DueDiligenceHandler
		panoramaHandler            *handlers.PanoramaHandler
		setorHandler               *handlers.SetorHandler
		riscoAmbientalHandler      *handlers.RiscoAmbientalHandler
		complianceEleitoralHandler *handlers.ComplianceEleitoralHandler
		creditoScoreHandler        *handlers.CreditoScoreHandler
		municipioHandler           *handlers.MunicipioHandler
		fundoAnaliseHandler        *handlers.FundoAnaliseHandler
		// Premium composite handlers (Phase 12)
		perfilCompletoHandler  *handlers.PerfilCompletoHandler
		carteiraRiscoHandler   *handlers.CarteiraRiscoHandler
		redeInfluenciaHandler  *handlers.RedeInfluenciaHandler
		litigioRiscoHandler    *handlers.LitigioRiscoHandler
		competicaoHandler      *handlers.CompeticaoHandler
		mercadoTrabalhoHandler *handlers.MercadoTrabalhoHandler
		regulacaoSetorHandler  *handlers.RegulacaoSetorHandler
		esgHandler             *handlers.ESGHandler
	)
	if store != nil {
		bcbHandler = handlers.NewBCBHandler(store)
		ecoHandler = handlers.NewEconomiaHandler(store)
		mercHandler = handlers.NewMercadoHandler(store)
		transHandler = handlers.NewTransparenciaHandler(store)
		saudeHandler = handlers.NewSaudeHandler(store)
		energiaHandler = handlers.NewEnergiaHandler(store)
		ambientalHandler = handlers.NewAmbientalHandler(store)
		transporteHandler = handlers.NewTransporteHandler(store)
		transportadoresHandler = handlers.NewTransportadoresHandler(store)
		titulosHandler = handlers.NewTitulosHandler(store)
		comercioHandler = handlers.NewComercioHandler(store)
		educacaoHandler = handlers.NewEducacaoHandler(store)
		empregoHandler = handlers.NewEmpregoHandler(store)
		// Premium handlers
		dueDiligenceHandler = handlers.NewDueDiligenceHandler(cnpjCollector, cguCollector, djCollector, store)
		panoramaHandler = handlers.NewPanoramaHandler(store)
		setorHandler = handlers.NewSetorHandler(cnpjCollector, store)
		riscoAmbientalHandler = handlers.NewRiscoAmbientalHandler(store)
		complianceEleitoralHandler = handlers.NewComplianceEleitoralHandler(cguCollector, djCollector, store)
		creditoScoreHandler = handlers.NewCreditoScoreHandler(cnpjCollector, cguCollector, djCollector, store)
		municipioHandler = handlers.NewMunicipioHandler(store)
		fundoAnaliseHandler = handlers.NewFundoAnaliseHandler(store)
		// Phase 12: Premium composite handlers
		perfilCompletoHandler = handlers.NewPerfilCompletoHandler(cnpjCollector, cguCollector, djCollector, store)
		carteiraRiscoHandler = handlers.NewCarteiraRiscoHandler(cnpjCollector, cguCollector, store)
		redeInfluenciaHandler = handlers.NewRedeInfluenciaHandler(cnpjCollector, store)
		litigioRiscoHandler = handlers.NewLitigioRiscoHandler(djCollector, cnpjCollector, store)
		competicaoHandler = handlers.NewCompeticaoHandler(store)
		mercadoTrabalhoHandler = handlers.NewMercadoTrabalhoHandler(store)
		regulacaoSetorHandler = handlers.NewRegulacaoSetorHandler(store)
		esgHandler = handlers.NewESGHandler(cnpjCollector, cguCollector, store)
	}

	// MCP server (invokes handlers directly, no HTTP loopback)
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + serverPort()
	}
	mcpDeps := &mcp.HandlerDeps{
		// On-demand handlers (always available)
		Empresas:    empHandler.GetEmpresa,
		Compliance:  compHandler.GetCompliance,
		Judicial:    judicialHand.GetProcessos,
		DOU:         douHandler.GetBusca,
		Orcamento:   orcamentoHandler.GetDespesas,
		TCU:         tcuHandler.GetCertidao,
		Legislativo: legislativoHandler.GetDeputados,
		PNCP:        pncpHandler.GetOrgaos,
	}
	// Store-backed handlers (only when DB is connected)
	if bcbHandler != nil {
		mcpDeps.BCBSelic = bcbHandler.GetSelic
		mcpDeps.BCBCambio = bcbHandler.GetCambio
	}
	if ecoHandler != nil {
		mcpDeps.EconomiaIPCA = ecoHandler.GetIPCA
		mcpDeps.EconomiaPIB = ecoHandler.GetPIB
	}
	if mercHandler != nil {
		mcpDeps.MercadoAcoes = mercHandler.GetAcoes
	}
	if energiaHandler != nil {
		mcpDeps.Energia = energiaHandler.GetTarifas
	}
	if saudeHandler != nil {
		mcpDeps.Saude = saudeHandler.GetMedicamento
	}
	mcpSrv := mcp.NewServer(mcpDeps)
	sseServer := mcpserver.NewSSEServer(mcpSrv.MCPServer(),
		mcpserver.WithBaseURL(baseURL+"/mcp"),
	)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// HEAD → GET: rewrite HEAD requests so x402 middleware returns 402 (not 405).
	// Per RFC 7231, HEAD must return the same headers as GET with no body.
	// x402 agents use HEAD to probe payment requirements without downloading the body.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				r.Method = http.MethodGet
				next.ServeHTTP(&headResponseWriter{ResponseWriter: w}, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Proxy TLS detection: behind Railway/Cloudflare r.TLS is nil because
	// TLS terminates at the proxy. The x402-go SDK checks r.TLS to decide
	// the scheme for the resource URL. This middleware sets r.TLS from the
	// X-Forwarded-Proto header so 402 responses emit https:// URLs.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "https" {
				r.TLS = &tls.ConnectionState{}
			}
			next.ServeHTTP(w, r)
		})
	})

	// Security headers
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			next.ServeHTTP(w, r)
		})
	})

	// CORS — restrict in production, allow all in dev
	allowedOrigins := []string{"https://databr.api.br", "https://*.up.railway.app"}
	if os.Getenv("RAILWAY_ENVIRONMENT") == "" && os.Getenv("FLY_APP_NAME") == "" {
		allowedOrigins = []string{"*"} // dev mode
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-PAYMENT", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Payment-Required", "Content-Length", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Rate limiting (100 req/min per IP)
	r.Use(httprate.Limit(
		100,
		1*time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			handlers.RateLimitExceeded(w)
		}),
	))

	// Query logging middleware
	r.Use(handlers.QueryLogMiddleware)

	// x402 discovery document — public, no payment required
	r.Get("/.well-known/x402", x402pkg.WellKnownHandler(x402Cfg))

	// Health check with DB + Redis verification
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		healthCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{}
		status := "ok"
		httpCode := http.StatusOK

		if pool != nil {
			if err := pool.Ping(healthCtx); err != nil {
				slog.Error("health check DB ping failed", "error", err)
				checks["database"] = "error"
				status = "degraded"
				httpCode = http.StatusServiceUnavailable
			} else {
				checks["database"] = "ok"
			}
		} else {
			checks["database"] = "not configured"
		}

		if cacher != nil {
			if err := cacher.Set(healthCtx, "health:ping", "ok", 10*time.Second); err != nil {
				checks["redis"] = "error"
				if status == "ok" {
					status = "degraded"
				}
			} else {
				checks["redis"] = "ok"
			}
		} else {
			checks["redis"] = "not configured"
		}

		resp := map[string]any{
			"status":         status,
			"version":        domain.Version,
			"uptime_seconds": int(time.Since(startTime).Seconds()),
			"checks":         checks,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpCode)
		json.NewEncoder(w).Encode(resp)
	})

	// Readiness probe for Railway/k8s
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if pool != nil {
			rctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
			defer cancel()
			if err := pool.Ping(rctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]bool{"ready": false})
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ready": true})
	})

	// Favicon — return 204 No Content to prevent 402 from x402 middleware.
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// SEO: robots.txt and sitemap.xml
	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("User-agent: *\nAllow: /\nAllow: /docs\nDisallow: /v1/\nDisallow: /mcp\n\nSitemap: https://databr.api.br/sitemap.xml\n"))
	})
	r.Get("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://databr.api.br/</loc><changefreq>weekly</changefreq><priority>1.0</priority></url>
  <url><loc>https://databr.api.br/docs</loc><changefreq>weekly</changefreq><priority>0.9</priority></url>
</urlset>`))
	})

	// Landing page
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://fonts.gstatic.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: https:; connect-src 'self'")
		serveEmbedded(w, docfs.Static, "landing.html", "text/html; charset=utf-8")
	})

	// API Documentation (Scalar UI)
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		// Relax CSP for docs page so Scalar CDN scripts/styles load correctly.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"font-src 'self' https://cdn.jsdelivr.net; "+
				"img-src 'self' data: https:; connect-src 'self'")
		serveEmbedded(w, docfs.Static, "scalar.html", "text/html; charset=utf-8")
	})
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		serveEmbedded(w, docfs.Static, "openapi.yaml", "text/yaml; charset=utf-8")
	})

	// /v1 API routes, grouped by x402 price tier
	r.Route("/v1", func(r chi.Router) {
		// $0.003 — basic lookups: company data, BCB rates, economic indicators, tesouro
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.003"))
			r.Use(cache.NewCacheMiddleware(cacher, 1*time.Hour))
			r.Get("/empresas/{cnpj}", empHandler.GetEmpresa)
			r.Get("/empresas/{cnpj}/socios", empHandler.GetSocios)
			r.Get("/empresas/{cnpj}/simples", empHandler.GetSimples)
			r.Get("/endereco/{cep}", enderecoHandler.GetEndereco)
			r.Get("/tesouro/rreo", tesouroHand.GetRREO)
			r.Get("/compliance/ceis/{cnpj}", compHandler.GetCEIS)
			r.Get("/compliance/cnep/{cnpj}", compHandler.GetCNEP)
			r.Get("/compliance/cepim/{cnpj}", compHandler.GetCEPIM)
			r.Get("/transparencia/contratos", transparenciaFedHandler.GetContratos)
			r.Get("/transparencia/servidores", transparenciaFedHandler.GetServidores)
			r.Get("/transparencia/beneficios", transparenciaFedHandler.GetBolsaFamilia)
			r.Get("/transparencia/cartoes", transparenciaFedHandler.GetCartoes)
			r.Get("/ibge/municipio/{ibge}", ibgeHandler.GetMunicipio)
			r.Get("/ibge/municipios/{uf}", ibgeHandler.GetMunicipiosPorUF)
			r.Get("/ibge/estados", ibgeHandler.GetEstados)
			r.Get("/ibge/regioes", ibgeHandler.GetRegioes)
			r.Get("/ibge/cnae/{codigo}", ibgeHandler.GetCNAE)
			r.Get("/legislativo/deputados", legislativoHandler.GetDeputados)
			r.Get("/legislativo/deputados/{id}", legislativoHandler.GetDeputado)
			r.Get("/legislativo/proposicoes", legislativoHandler.GetProposicoes)
			r.Get("/legislativo/votacoes", legislativoHandler.GetVotacoes)
			r.Get("/legislativo/partidos", legislativoHandler.GetPartidos)
			r.Get("/legislativo/senado/senadores", legislativoHandler.GetSenadores)
			r.Get("/legislativo/senado/materias", legislativoHandler.GetMateriasSenado)
			r.Get("/legislativo/eventos", legislativoHandler.GetEventos)
			r.Get("/legislativo/comissoes", legislativoHandler.GetComissoes)
			r.Get("/ipea/serie/{codigo}", ipeaHandler.GetSerie)
			r.Get("/ipea/busca", ipeaHandler.GetBusca)
			r.Get("/ipea/temas", ipeaHandler.GetTemas)
			r.Get("/bcb/indicadores/{serie}", proxyBCBHandler.GetIndicadores)
			r.Get("/bcb/capitais", proxyBCBHandler.GetCapitais)
			r.Get("/bcb/sml", proxyBCBHandler.GetSML)
			r.Get("/ibge/pnad", ibgeHandler.GetPNAD)
			r.Get("/ibge/inpc", ibgeHandler.GetINPC)
			r.Get("/ibge/pim", ibgeHandler.GetPIM)
			r.Get("/ibge/populacao", ibgeHandler.GetPopulacao)
			r.Get("/ibge/ipca15", ibgeHandler.GetIPCA15)
			r.Get("/tesouro/entes", tesouroHand.GetEntes)
			r.Get("/tesouro/rgf", tesouroHand.GetRGF)
			r.Get("/tesouro/dca", tesouroHand.GetDCA)
			r.Get("/legislativo/frentes", legislativoHandler.GetFrentes)
			r.Get("/legislativo/blocos", legislativoHandler.GetBlocos)
			r.Get("/legislativo/deputados/{id}/despesas", legislativoHandler.GetDespesas)
			r.Get("/transparencia/ceaf/{cnpj}", transparenciaFedHandler.GetCEAF)
			r.Get("/transparencia/viagens", transparenciaFedHandler.GetViagens)
			r.Get("/transparencia/emendas", transparenciaFedHandler.GetEmendas)
			r.Get("/transparencia/obras", transparenciaFedHandler.GetObras)
			r.Get("/transparencia/transferencias", transparenciaFedHandler.GetTransferencias)
			r.Get("/transparencia/pensionistas", transparenciaFedHandler.GetPensionistas)
			r.Get("/pncp/orgaos", pncpHandler.GetOrgaos)
			r.Get("/bcb/ifdata", proxyBCBHandler.GetIFData)
			r.Get("/bcb/base-monetaria", proxyBCBHandler.GetBaseMonetaria)
			r.Get("/ibge/pmc", ibgeHandler.GetPMC)
			r.Get("/ibge/pms", ibgeHandler.GetPMS)
			r.Get("/eleicoes/bens", tseExtrasHandler.GetBens)
			r.Get("/eleicoes/doacoes", tseExtrasHandler.GetDoacoes)
			r.Get("/eleicoes/resultados", tseExtrasHandler.GetResultados)
			r.Get("/energia/combustiveis", tseExtrasHandler.GetCombustiveis)
			r.Get("/saude/planos", ansHandler.GetPlanos)
			r.Get("/tcu/acordaos", tcuHandler.GetAcordaos)
			r.Get("/tcu/certidao/{cnpj}", tcuHandler.GetCertidao)
			r.Get("/tcu/inabilitados", tcuHandler.GetInabilitados)
			r.Get("/tcu/inabilitados/{cpf}", tcuHandler.GetInabilitadoByCPF)
			r.Get("/tcu/contratos", tcuHandler.GetContratos)
			r.Get("/orcamento/despesas", orcamentoHandler.GetDespesas)
			r.Get("/orcamento/funcional-programatica", orcamentoHandler.GetFuncionalProgramatica)
			r.Get("/orcamento/documento/{codigo}", orcamentoHandler.GetDocumento)
			if bcbHandler != nil {
				r.Get("/bcb/selic", bcbHandler.GetSelic)
				r.Get("/bcb/cambio/{moeda}", bcbHandler.GetCambio)
				r.Get("/bcb/pix/estatisticas", bcbHandler.GetPIX)
				r.Get("/bcb/credito", bcbHandler.GetCredito)
				r.Get("/bcb/reservas", bcbHandler.GetReservas)
				r.Get("/bcb/taxas-credito", bcbHandler.GetTaxasCredito)
			}
			if titulosHandler != nil {
				r.Get("/tesouro/titulos", titulosHandler.GetTitulos)
			}
			if ecoHandler != nil {
				r.Get("/economia/ipca", ecoHandler.GetIPCA)
				r.Get("/economia/pib", ecoHandler.GetPIB)
				r.Get("/economia/focus", ecoHandler.GetFocus)
			}
			if transHandler != nil {
				r.Get("/transparencia/licitacoes", transHandler.GetLicitacoes)
				r.Get("/eleicoes/candidatos", transHandler.GetCandidatos)
			}
			if saudeHandler != nil {
				r.Get("/saude/medicamentos/{registro}", saudeHandler.GetMedicamento)
			}
			if energiaHandler != nil {
				r.Get("/energia/tarifas", energiaHandler.GetTarifas)
			}
			if mercHandler != nil {
				r.Get("/mercado/fatos-relevantes/{protocolo}", mercHandler.GetFatosById)
			}
			if transporteHandler != nil {
				r.Get("/transporte/aeronaves/{prefixo}", transporteHandler.GetAeronave)
			}
			if transportadoresHandler != nil {
				r.Get("/transporte/transportadores/{rntrc}", transportadoresHandler.GetTransportador)
			}
		})

		// $0.005 — standard: B3 stock quotes, CVM fatos relevantes, INPE deforestation data, budget documents
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.005"))
			r.Use(cache.NewCacheMiddleware(cacher, 15*time.Minute))
			r.Get("/orcamento/documentos", orcamentoHandler.GetDocumentos)
			r.Get("/orcamento/favorecidos", orcamentoHandler.GetFavorecidos)
			if mercHandler != nil {
				r.Get("/mercado/acoes/{ticker}", mercHandler.GetAcoes)
				r.Get("/mercado/fatos-relevantes", mercHandler.GetFatosRelevantes)
				r.Get("/mercado/fundos/{cnpj}/cotas", mercHandler.GetCotasByCNPJ)
			}
			if ambientalHandler != nil {
				r.Get("/ambiental/desmatamento", ambientalHandler.GetDesmatamento)
				r.Get("/ambiental/prodes", ambientalHandler.GetProdes)
			}
			if transporteHandler != nil {
				r.Get("/transporte/aeronaves", transporteHandler.GetAeronaves)
			}
			if transportadoresHandler != nil {
				r.Get("/transporte/transportadores", transportadoresHandler.GetTransportadoresByCNPJ)
			}
			if transporteHandler != nil {
				r.Get("/transporte/acidentes", transporteHandler.GetAcidentes)
			}
			if comercioHandler != nil {
				r.Get("/comercio/exportacoes", comercioHandler.GetExportacoes)
				r.Get("/comercio/importacoes", comercioHandler.GetImportacoes)
			}
			if mercHandler != nil {
				r.Get("/mercado/indices/ibovespa", mercHandler.GetIndicesIbovespa)
			}
			if educacaoHandler != nil {
				r.Get("/educacao/censo-escolar", educacaoHandler.GetCensoEscolar)
			}
			if empregoHandler != nil {
				r.Get("/emprego/rais", empregoHandler.GetRAIS)
				r.Get("/emprego/caged", empregoHandler.GetCAGED)
			}
			if energiaHandler != nil {
				r.Get("/energia/geracao", energiaHandler.GetGeracao)
				r.Get("/energia/carga", energiaHandler.GetCarga)
			}
			if ambientalHandler != nil {
				r.Get("/ambiental/uso-solo", ambientalHandler.GetUsoSolo)
				r.Get("/ambiental/embargos", ambientalHandler.GetEmbargos)
			}
		})

		// $0.007 — enhanced: compliance via empresa, DOU/diários search, cross-references
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.007"))
			r.Use(cache.NewCacheMiddleware(cacher, 30*time.Minute))
			r.Get("/empresas/{cnpj}/compliance", compHandler.GetCompliance)
			r.Get("/dou/busca", douHandler.GetBusca)
			r.Get("/diarios/busca", douHandler.GetDiarios)
			if setorHandler != nil {
				r.Get("/empresas/{cnpj}/setor", setorHandler.GetSetor)
			}
			if riscoAmbientalHandler != nil {
				r.Get("/ambiental/risco/{municipio}", riscoAmbientalHandler.GetRiscoAmbiental)
			}
			if complianceEleitoralHandler != nil {
				r.Get("/eleicoes/compliance/{cpf_cnpj}", complianceEleitoralHandler.GetComplianceEleitoral)
			}
			if municipioHandler != nil {
				r.Get("/municipios/{codigo}/perfil", municipioHandler.GetMunicipioPerfil)
			}
		})

		// $0.010 — premium: full compliance, CVM fund data, fund analysis, credit score
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.010"))
			r.Use(cache.NewCacheMiddleware(cacher, 30*time.Minute))
			r.Get("/compliance/{cnpj}", compHandler.GetCompliance)
			if mercHandler != nil {
				r.Get("/mercado/fundos/{cnpj}", mercHandler.GetFundos)
			}
			if fundoAnaliseHandler != nil {
				r.Get("/mercado/fundos/{cnpj}/analise", fundoAnaliseHandler.GetFundoAnalise)
			}
			if creditoScoreHandler != nil {
				r.Get("/credito/score/{cnpj}", creditoScoreHandler.GetCreditoScore)
			}
		})

		// $0.015 — advanced: judicial process search, economic panorama, labor market
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.015"))
			r.Use(cache.NewCacheMiddleware(cacher, 1*time.Hour))
			r.Get("/judicial/processos/{doc}", judicialHand.GetProcessos)
			if panoramaHandler != nil {
				r.Get("/economia/panorama", panoramaHandler.GetPanorama)
			}
			if mercadoTrabalhoHandler != nil {
				r.Get("/mercado-trabalho/{uf}/analise", mercadoTrabalhoHandler.GetMercadoTrabalho)
			}
		})

		// $0.020 — composite: perfil completo, sector regulation
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.020"))
			r.Use(cache.NewCacheMiddleware(cacher, 1*time.Hour))
			if perfilCompletoHandler != nil {
				r.Get("/empresas/{cnpj}/perfil-completo", perfilCompletoHandler.GetPerfilCompleto)
			}
			if regulacaoSetorHandler != nil {
				r.Get("/setor/{cnae}/regulacao", regulacaoSetorHandler.GetRegulacaoSetor)
			}
		})

		// $0.030 — deep analysis: competition, ESG scoring, litigation risk
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.030"))
			r.Use(cache.NewCacheMiddleware(cacher, 1*time.Hour))
			if competicaoHandler != nil {
				r.Get("/mercado/{cnae}/competicao", competicaoHandler.GetCompeticao)
			}
			if esgHandler != nil {
				r.Get("/ambiental/empresa/{cnpj}/esg", esgHandler.GetESG)
			}
			if litigioRiscoHandler != nil {
				r.Get("/litigio/{cnpj}/risco", litigioRiscoHandler.GetLitigioRisco)
			}
		})

		// $0.050 — network/influence analysis
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.050"))
			r.Use(cache.NewCacheMiddleware(cacher, 1*time.Hour))
			if redeInfluenciaHandler != nil {
				r.Get("/rede/{cnpj}/influencia", redeInfluenciaHandler.GetRedeInfluencia)
			}
		})

		// $0.075 — due diligence
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.075"))
			r.Use(cache.NewCacheMiddleware(cacher, 1*time.Hour))
			if dueDiligenceHandler != nil {
				r.Get("/empresas/{cnpj}/duediligence", dueDiligenceHandler.GetDueDiligence)
			}
		})

		// $0.150 — portfolio risk (batch, POST — no cache)
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.150"))
			if carteiraRiscoHandler != nil {
				r.Post("/carteira/risco", carteiraRiscoHandler.PostCarteiraRisco)
			}
		})
	})

	// MCP server (SSE transport) — protected by x402.
	// Price set to $0.015 (max of any tool proxied through MCP: judicial/processos).
	r.Group(func(r chi.Router) {
		r.Use(optionalX402(x402Cfg, "0.015"))
		r.Mount("/mcp", sseServer)
	})

	// Server with timeouts and graceful shutdown
	addr := ":" + serverPort()
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("databr API listening",
			"addr", addr,
			"wallet", maskWallet(x402Cfg.WalletAddress),
			"network", x402Cfg.Network,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGTERM/SIGINT
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	slog.Info("shutdown signal received, draining connections")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	}
	slog.Info("server stopped gracefully")
}

// serverPort returns the HTTP port from PORT env var, defaulting to 8080.
func serverPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

// networkName converts a short name or EIP-155 chain ID to CAIP-2 format.
// Accepts "base", "base-sepolia", "eip155:8453", "eip155:84532".
// Defaults to "eip155:84532" (Base Sepolia testnet) when no network is configured.
func networkName(eipNetwork string) string {
	switch {
	case eipNetwork == "eip155:8453":
		return "eip155:8453"
	case eipNetwork == "base",
		strings.Contains(eipNetwork, "8453") && !strings.Contains(eipNetwork, "84532"):
		return "eip155:8453"
	default:
		return "eip155:84532"
	}
}

// optionalX402 returns a pass-through middleware when wallet address is not set (dev mode).
// When wallet is set, creates a real x402 payment gate for the given USDC price.
// In production (Railway/Fly.io), exits if WALLET_ADDRESS is not set.
func optionalX402(cfg x402pkg.MiddlewareConfig, priceUSDC string) func(http.Handler) http.Handler {
	if cfg.WalletAddress == "" {
		if os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("FLY_APP_NAME") != "" {
			slog.Error("WALLET_ADDRESS must be set in production")
			os.Exit(1)
		}
		slog.Warn("WALLET_ADDRESS not set, x402 payment disabled (dev mode)")
		// Still inject price into context so handlers can read it via PriceFromRequest.
		return x402pkg.PriceInjectorMiddleware(priceUSDC)
	}
	return x402pkg.NewPricedMiddleware(cfg, priceUSDC)
}

// maskWallet returns the first 6 + last 4 chars of the wallet for log output.
func maskWallet(addr string) string {
	if len(addr) < 10 {
		return "(not set)"
	}
	return addr[:6] + "…" + addr[len(addr)-4:]
}

// headResponseWriter wraps http.ResponseWriter to discard the response body for HEAD requests.
// Headers (including 402 payment requirements) are forwarded normally.
type headResponseWriter struct {
	http.ResponseWriter
}

func (h *headResponseWriter) Write([]byte) (int, error) { return 0, nil }

// serveEmbedded reads a file from an embed.FS and writes it to the response.
func serveEmbedded(w http.ResponseWriter, fsys embed.FS, name, contentType string) {
	data, err := fsys.ReadFile(name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}
