package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	x402sdk "github.com/coinbase/x402/go"
	x402http "github.com/coinbase/x402/go/http"
	evmexact "github.com/coinbase/x402/go/mechanisms/evm/exact/client"
	"github.com/coinbase/x402/go/signers/evm"
)

func main() {
	privateKey := os.Getenv("WALLET_PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("WALLET_PRIVATE_KEY env var required")
	}

	baseURL := os.Getenv("DATABR_URL")
	if baseURL == "" {
		baseURL = "https://databr-production.up.railway.app"
	}

	// Create EVM signer from private key
	signer, err := evm.NewClientSignerFromPrivateKey(privateKey)
	if err != nil {
		log.Fatalf("signer: %v", err)
	}
	fmt.Printf("Wallet address: %s\n", signer.Address())

	// Create x402 client with EVM exact scheme for Base mainnet
	x402Client := x402sdk.Newx402Client()
	evmScheme := evmexact.NewExactEvmScheme(signer)
	x402Client.Register("eip155:8453", evmScheme)

	// Wrap standard HTTP client with x402 payment handling
	httpClient := x402http.WrapHTTPClientWithPayment(http.DefaultClient, x402http.NewClient(x402Client))

	// All DataBR endpoints to bootstrap for Bazaar indexing.
	// Each unique URL that goes through CDP facilitator settlement gets indexed.
	// Parameterized routes use example values to trigger unique URLs.
	endpoints := []string{
		// === $0.001 tier ===
		// BCB
		"/v1/bcb/selic",
		"/v1/bcb/cambio/USD",
		"/v1/bcb/cambio/EUR",
		"/v1/bcb/cambio/GBP",
		"/v1/bcb/cambio/JPY",
		"/v1/bcb/cambio/CNY",
		"/v1/bcb/pix/estatisticas",
		"/v1/bcb/credito",
		"/v1/bcb/reservas",
		"/v1/bcb/taxas-credito",
		"/v1/bcb/indicadores/432",   // Selic diária
		"/v1/bcb/indicadores/11753", // CDI
		"/v1/bcb/capitais",
		"/v1/bcb/sml",
		"/v1/bcb/ifdata",
		"/v1/bcb/base-monetaria",
		// Economia
		"/v1/economia/ipca",
		"/v1/economia/pib",
		"/v1/economia/focus",
		// Empresas
		"/v1/empresas/00000000000191",          // Banco do Brasil
		"/v1/empresas/00000000000191/socios",   // QSA
		"/v1/empresas/00000000000191/simples",  // Simples Nacional
		"/v1/empresas/33000167000101",          // Petrobras
		"/v1/empresas/33000167000101/socios",
		// Endereço
		"/v1/endereco/01001000", // São Paulo centro
		"/v1/endereco/20040020", // Rio de Janeiro centro
		// Tesouro
		"/v1/tesouro/rreo",
		"/v1/tesouro/entes",
		"/v1/tesouro/rgf",
		"/v1/tesouro/dca",
		"/v1/tesouro/titulos",
		// Compliance
		"/v1/compliance/ceis/00000000000191",
		"/v1/compliance/cnep/00000000000191",
		"/v1/compliance/cepim/00000000000191",
		// Transparência
		"/v1/transparencia/contratos",
		"/v1/transparencia/servidores",
		"/v1/transparencia/beneficios",
		"/v1/transparencia/cartoes",
		"/v1/transparencia/ceaf/00000000000191",
		"/v1/transparencia/viagens",
		"/v1/transparencia/emendas",
		"/v1/transparencia/obras",
		"/v1/transparencia/transferencias",
		"/v1/transparencia/pensionistas",
		"/v1/transparencia/licitacoes",
		// IBGE
		"/v1/ibge/municipio/3550308", // São Paulo
		"/v1/ibge/municipio/3304557", // Rio de Janeiro
		"/v1/ibge/municipios/SP",
		"/v1/ibge/municipios/RJ",
		"/v1/ibge/estados",
		"/v1/ibge/regioes",
		"/v1/ibge/cnae/6110803",
		"/v1/ibge/pnad",
		"/v1/ibge/inpc",
		"/v1/ibge/pim",
		"/v1/ibge/populacao",
		"/v1/ibge/ipca15",
		"/v1/ibge/pmc",
		"/v1/ibge/pms",
		// Legislativo
		"/v1/legislativo/deputados",
		"/v1/legislativo/deputados/204554",
		"/v1/legislativo/deputados/204554/despesas",
		"/v1/legislativo/proposicoes",
		"/v1/legislativo/votacoes",
		"/v1/legislativo/partidos",
		"/v1/legislativo/senado/senadores",
		"/v1/legislativo/senado/materias",
		"/v1/legislativo/eventos",
		"/v1/legislativo/comissoes",
		"/v1/legislativo/frentes",
		"/v1/legislativo/blocos",
		// IPEA
		"/v1/ipea/serie/BM12_TJOVER12",
		"/v1/ipea/busca?termo=selic",
		"/v1/ipea/temas",
		// PNCP
		"/v1/pncp/orgaos",
		// Eleições
		"/v1/eleicoes/candidatos",
		"/v1/eleicoes/bens",
		"/v1/eleicoes/doacoes",
		"/v1/eleicoes/resultados",
		// Energia
		"/v1/energia/combustiveis",
		"/v1/energia/tarifas",
		// Saúde
		"/v1/saude/medicamentos/100650172",
		"/v1/saude/planos",
		// Transporte
		"/v1/transporte/aeronaves/PTRMI",
		"/v1/transporte/transportadores/00000000000191",
		// Mercado (single item)
		"/v1/mercado/fatos-relevantes/1",
		// TCU
		"/v1/tcu/acordaos",
		"/v1/tcu/certidao/00000000000191",
		"/v1/tcu/inabilitados",
		"/v1/tcu/contratos",
		// Orçamento
		"/v1/orcamento/despesas?orgao=26000",
		"/v1/orcamento/funcional-programatica",
		"/v1/orcamento/documento/1",

		// === $0.002 tier ===
		"/v1/mercado/acoes/PETR4",
		"/v1/mercado/acoes/VALE3",
		"/v1/mercado/fatos-relevantes",
		"/v1/mercado/fundos/97929213000107/cotas",
		"/v1/ambiental/desmatamento",
		"/v1/ambiental/prodes",
		"/v1/transporte/aeronaves",
		"/v1/transporte/transportadores",
		"/v1/orcamento/documentos",
		"/v1/orcamento/favorecidos",
		"/v1/comercio/exportacoes",
		"/v1/comercio/importacoes",
		"/v1/mercado/indices/ibovespa",
		"/v1/educacao/censo-escolar",
		"/v1/transporte/acidentes",
		"/v1/emprego/rais",
		"/v1/emprego/caged",
		"/v1/energia/geracao",
		"/v1/energia/carga",
		"/v1/ambiental/uso-solo",
		"/v1/ambiental/embargos",

		// === $0.003 tier ===
		"/v1/empresas/00000000000191/compliance",
		"/v1/empresas/33000167000101/compliance",
		"/v1/dou/busca?termo=licitacao",
		"/v1/diarios/busca?termo=prefeitura",
		"/v1/empresas/00000000000191/setor",
		"/v1/ambiental/risco/3550308",
		"/v1/municipios/3550308/perfil",

		// === $0.005 tier ===
		"/v1/compliance/00000000000191",
		"/v1/mercado/fundos/97929213000107",
		"/v1/mercado/fundos/97929213000107/analise",
		"/v1/credito/score/00000000000191",

		// === $0.010 tier ===
		"/v1/judicial/processos/00000000000191",
		"/v1/economia/panorama",
		"/v1/mercado-trabalho/SP/analise",

		// === $0.015 tier ===
		"/v1/empresas/00000000000191/perfil-completo",
		"/v1/setor/6110803/regulacao",

		// === $0.020 tier ===
		"/v1/mercado/6110803/competicao",
		"/v1/ambiental/empresa/00000000000191/esg",
		"/v1/litigio/00000000000191/risco",

		// === $0.030 tier ===
		"/v1/rede/00000000000191/influencia",

		// === $0.050 tier ===
		"/v1/empresas/00000000000191/duediligence",
	}

	// Note: skipping /v1/carteira/risco ($0.100, POST with body)

	fmt.Printf("\nBootstrapping %d endpoints for Bazaar indexing\n", len(endpoints))
	fmt.Printf("Estimated cost: see pricing tiers above\n\n")

	success, failed := 0, 0
	for i, ep := range endpoints {
		url := baseURL + ep
		fmt.Printf("[%d/%d] %s ... ", i+1, len(endpoints), ep)

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL: %v\n", err)
			failed++
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		payResp := resp.Header.Get("X-PAYMENT-RESPONSE")
		if resp.StatusCode == 200 && payResp != "" {
			fmt.Printf("OK (paid, %d bytes)\n", len(body))
			success++
		} else if resp.StatusCode == 200 {
			fmt.Printf("OK (cached, %d bytes)\n", len(body))
			success++
		} else {
			fmt.Printf("HTTP %d (%d bytes)\n", resp.StatusCode, len(body))
			if resp.StatusCode != 402 {
				failed++
			} else {
				failed++ // 402 means payment didn't go through
			}
		}

		// Throttle: 2 requests/sec to stay well under 100 req/min rate limit
		// and avoid overwhelming the facilitator
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total: %d | Success: %d | Failed: %d\n", len(endpoints), success, failed)
}
