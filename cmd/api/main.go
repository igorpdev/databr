package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	cnpjcol "github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/collectors/dou"
	"github.com/databr/api/internal/collectors/juridico"
	"github.com/databr/api/internal/collectors/tesouro"
	"github.com/databr/api/internal/collectors/transparencia"
	"github.com/databr/api/internal/handlers"
	"github.com/databr/api/internal/mcp"
	"github.com/databr/api/internal/repositories"
	x402pkg "github.com/databr/api/internal/x402"
	migpkg "github.com/databr/api/migrations"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file, using environment variables")
	}

	ctx := context.Background()

	// Database (optional — store-backed handlers degrade gracefully when nil)
	var store handlers.SourceStore
	if pool, err := repositories.NewPool(ctx); err != nil {
		log.Printf("DB unavailable (%v) — store-backed endpoints disabled", err)
	} else {
		if err := repositories.RunMigrations(ctx, pool, migpkg.FS); err != nil {
			log.Printf("WARNING: migrations failed: %v", err)
		}
		store = repositories.NewSourceRecordRepository(pool)
		defer pool.Close()
	}

	// x402 payment config (wallet required; set to empty string in dev = no-op middleware)
	x402Cfg := x402pkg.MiddlewareConfig{
		WalletAddress:  os.Getenv("WALLET_ADDRESS"),
		FacilitatorURL: os.Getenv("X402_FACILITATOR_URL"),
		Network:        networkName(os.Getenv("X402_NETWORK")),
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
	tesouroHand := handlers.NewTesouroHandler(tesouroCol)
	douHandler := handlers.NewDOUHandler(qdCollector)
	judicialHand := handlers.NewJudicialHandler(djCollector)

	// Store-backed handlers (only available when DB is connected)
	var (
		bcbHandler   *handlers.BCBHandler
		ecoHandler   *handlers.EconomiaHandler
		mercHandler  *handlers.MercadoHandler
		transHandler *handlers.TransparenciaHandler
	)
	if store != nil {
		bcbHandler = handlers.NewBCBHandler(store)
		ecoHandler = handlers.NewEconomiaHandler(store)
		mercHandler = handlers.NewMercadoHandler(store)
		transHandler = handlers.NewTransparenciaHandler(store)
	}

	// MCP server (proxies to this REST API via SSE transport)
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + serverPort()
	}
	mcpSrv := mcp.NewServer(baseURL)
	sseServer := mcpserver.NewSSEServer(mcpSrv.MCPServer(),
		mcpserver.WithBaseURL(baseURL+"/mcp"),
	)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// /v1 API routes, grouped by x402 price tier
	r.Route("/v1", func(r chi.Router) {
		// $0.001 — company data, BCB rates, economic indicators, tesouro
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.001"))
			r.Get("/empresas/{cnpj}", empHandler.GetEmpresa)
			r.Get("/tesouro/rreo", tesouroHand.GetRREO)
			if bcbHandler != nil {
				r.Get("/bcb/selic", bcbHandler.GetSelic)
				r.Get("/bcb/cambio/{moeda}", bcbHandler.GetCambio)
				r.Get("/bcb/pix/estatisticas", bcbHandler.GetPIX)
				r.Get("/bcb/credito", bcbHandler.GetCredito)
				r.Get("/bcb/reservas", bcbHandler.GetReservas)
			}
			if ecoHandler != nil {
				r.Get("/economia/ipca", ecoHandler.GetIPCA)
				r.Get("/economia/pib", ecoHandler.GetPIB)
			}
			if transHandler != nil {
				r.Get("/transparencia/licitacoes", transHandler.GetLicitacoes)
				r.Get("/eleicoes/candidatos", transHandler.GetCandidatos)
			}
		})

		// $0.002 — B3 stock quotes
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.002"))
			if mercHandler != nil {
				r.Get("/mercado/acoes/{ticker}", mercHandler.GetAcoes)
			}
		})

		// $0.003 — compliance via empresa sub-route, DOU search
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.003"))
			r.Get("/empresas/{cnpj}/compliance", compHandler.GetCompliance)
			r.Get("/dou/busca", douHandler.GetBusca)
		})

		// $0.005 — full compliance check, CVM fund data
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.005"))
			r.Get("/compliance/{cnpj}", compHandler.GetCompliance)
			if mercHandler != nil {
				r.Get("/mercado/fundos/{cnpj}", mercHandler.GetFundos)
			}
		})

		// $0.010 — judicial process search (DataJud CNJ)
		r.Group(func(r chi.Router) {
			r.Use(optionalX402(x402Cfg, "0.010"))
			r.Get("/judicial/processos/{doc}", judicialHand.GetProcessos)
		})
	})

	// MCP server (SSE transport) — mounted after /v1 to avoid path conflicts
	r.Mount("/mcp", sseServer)

	addr := ":" + serverPort()
	log.Printf("databr API listening on %s (wallet=%s, network=%s)",
		addr,
		maskWallet(x402Cfg.WalletAddress),
		x402Cfg.Network,
	)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

// serverPort returns the HTTP port from PORT env var, defaulting to 8080.
func serverPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

// networkName converts an EIP-155 chain ID string to the short name used by x402-go.
// Defaults to base-sepolia (testnet) when no network is configured.
func networkName(eipNetwork string) string {
	switch {
	case strings.Contains(eipNetwork, "8453") && !strings.Contains(eipNetwork, "84532"):
		return "base"
	default:
		return "base-sepolia"
	}
}

// optionalX402 returns a pass-through middleware when wallet address is not set (dev mode).
// When wallet is set, creates a real x402 payment gate for the given USDC price.
func optionalX402(cfg x402pkg.MiddlewareConfig, priceUSDC string) func(http.Handler) http.Handler {
	if cfg.WalletAddress == "" {
		return func(next http.Handler) http.Handler { return next }
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
