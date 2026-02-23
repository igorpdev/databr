// Package x402 provides payment middleware and pricing for the DataBR API.
// It uses the official Coinbase x402 SDK for facilitator communication and
// V2 payment types with Bazaar discovery extensions.
package x402

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// priceTable maps Chi route patterns to their USDC prices (as decimal strings).
var priceTable = map[string]string{
	// $0.001 — company data, BCB, economic indicators, tesouro, legislativo, IPEA, IBGE
	"/v1/empresas/{cnpj}":                       "0.001",
	"/v1/empresas/{cnpj}/socios":                "0.001",
	"/v1/empresas/{cnpj}/simples":               "0.001",
	"/v1/endereco/{cep}":                        "0.001",
	"/v1/bcb/selic":                              "0.001",
	"/v1/bcb/cambio/{moeda}":                    "0.001",
	"/v1/bcb/pix/estatisticas":                  "0.001",
	"/v1/bcb/credito":                           "0.001",
	"/v1/bcb/reservas":                          "0.001",
	"/v1/bcb/taxas-credito":                     "0.001",
	"/v1/bcb/indicadores/{serie}":               "0.001",
	"/v1/bcb/capitais":                          "0.001",
	"/v1/bcb/sml":                               "0.001",
	"/v1/bcb/ifdata":                            "0.001",
	"/v1/bcb/base-monetaria":                    "0.001",
	"/v1/economia/ipca":                         "0.001",
	"/v1/economia/pib":                          "0.001",
	"/v1/economia/focus":                        "0.001",
	"/v1/tesouro/rreo":                          "0.001",
	"/v1/tesouro/entes":                         "0.001",
	"/v1/tesouro/rgf":                           "0.001",
	"/v1/tesouro/dca":                           "0.001",
	"/v1/tesouro/titulos":                       "0.001",
	"/v1/compliance/ceis/{cnpj}":                "0.001",
	"/v1/compliance/cnep/{cnpj}":                "0.001",
	"/v1/compliance/cepim/{cnpj}":               "0.001",
	"/v1/transparencia/contratos":               "0.001",
	"/v1/transparencia/servidores":              "0.001",
	"/v1/transparencia/beneficios":              "0.001",
	"/v1/transparencia/cartoes":                 "0.001",
	"/v1/transparencia/ceaf/{cnpj}":             "0.001",
	"/v1/transparencia/viagens":                 "0.001",
	"/v1/transparencia/emendas":                 "0.001",
	"/v1/transparencia/obras":                   "0.001",
	"/v1/transparencia/transferencias":          "0.001",
	"/v1/transparencia/pensionistas":            "0.001",
	"/v1/transparencia/licitacoes":              "0.001",
	"/v1/ibge/municipio/{ibge}":                 "0.001",
	"/v1/ibge/municipios/{uf}":                  "0.001",
	"/v1/ibge/estados":                          "0.001",
	"/v1/ibge/regioes":                          "0.001",
	"/v1/ibge/cnae/{codigo}":                    "0.001",
	"/v1/ibge/pnad":                             "0.001",
	"/v1/ibge/inpc":                             "0.001",
	"/v1/ibge/pim":                              "0.001",
	"/v1/ibge/populacao":                        "0.001",
	"/v1/ibge/ipca15":                           "0.001",
	"/v1/ibge/pmc":                              "0.001",
	"/v1/ibge/pms":                              "0.001",
	"/v1/legislativo/deputados":                 "0.001",
	"/v1/legislativo/deputados/{id}":            "0.001",
	"/v1/legislativo/deputados/{id}/despesas":   "0.001",
	"/v1/legislativo/proposicoes":               "0.001",
	"/v1/legislativo/votacoes":                  "0.001",
	"/v1/legislativo/partidos":                  "0.001",
	"/v1/legislativo/senado/senadores":          "0.001",
	"/v1/legislativo/senado/materias":           "0.001",
	"/v1/legislativo/eventos":                   "0.001",
	"/v1/legislativo/comissoes":                 "0.001",
	"/v1/legislativo/frentes":                   "0.001",
	"/v1/legislativo/blocos":                    "0.001",
	"/v1/ipea/serie/{codigo}":                   "0.001",
	"/v1/ipea/busca":                            "0.001",
	"/v1/ipea/temas":                            "0.001",
	"/v1/pncp/orgaos":                           "0.001",
	"/v1/eleicoes/candidatos":                   "0.001",
	"/v1/eleicoes/bens":                         "0.001",
	"/v1/eleicoes/doacoes":                      "0.001",
	"/v1/eleicoes/resultados":                   "0.001",
	"/v1/energia/combustiveis":                  "0.001",
	"/v1/energia/tarifas":                       "0.001",
	"/v1/saude/medicamentos/{registro}":         "0.001",
	"/v1/saude/planos":                          "0.001",
	"/v1/transporte/transportadores/{rntrc}":    "0.001",
	"/v1/transporte/aeronaves/{prefixo}":        "0.001",
	"/v1/mercado/fatos-relevantes/{protocolo}":  "0.001",
	// $0.002 — B3 stock quotes, CVM, INPE, transport lists, comex, educacao
	"/v1/mercado/acoes/{ticker}":       "0.002",
	"/v1/mercado/fatos-relevantes":     "0.002",
	"/v1/mercado/fundos/{cnpj}/cotas":  "0.002",
	"/v1/ambiental/desmatamento":       "0.002",
	"/v1/ambiental/prodes":             "0.002",
	"/v1/transporte/aeronaves":         "0.002",
	"/v1/transporte/transportadores":   "0.002",
	"/v1/comercio/exportacoes":         "0.002",
	"/v1/comercio/importacoes":         "0.002",
	"/v1/mercado/indices/ibovespa":     "0.002",
	"/v1/educacao/censo-escolar":       "0.002",
	"/v1/transporte/acidentes":         "0.002",
	"/v1/emprego/rais":                 "0.002",
	"/v1/emprego/caged":                "0.002",
	"/v1/energia/geracao":              "0.002",
	"/v1/energia/carga":                "0.002",
	"/v1/ambiental/uso-solo":           "0.002",
	"/v1/ambiental/embargos":           "0.002",
	// $0.003 — compliance via empresa sub-route, DOU/diários, environmental risk, electoral compliance
	"/v1/empresas/{cnpj}/compliance":           "0.003",
	"/v1/empresas/{cnpj}/setor":                "0.003",
	"/v1/dou/busca":                            "0.003",
	"/v1/diarios/busca":                        "0.003",
	"/v1/ambiental/risco/{municipio}":          "0.003",
	"/v1/eleicoes/compliance/{cpf_cnpj}":       "0.003",
	"/v1/municipios/{codigo}/perfil":           "0.003",
	// $0.005 — full compliance, CVM funds, judicial superior courts, score, analysis
	"/v1/compliance/{cnpj}":                    "0.005",
	"/v1/mercado/fundos/{cnpj}":                "0.005",
	"/v1/judicial/stf":                         "0.005",
	"/v1/judicial/stj":                         "0.005",
	"/v1/mercado/fundos/{cnpj}/analise":        "0.005",
	"/v1/credito/score/{cnpj}":                 "0.005",
	// $0.010 — judicial process search, panorama, due diligence
	"/v1/judicial/processos/{doc}":             "0.010",
	"/v1/economia/panorama":                    "0.010",
	// $0.015 — enhanced composite: perfil completo, regulação setorial
	"/v1/empresas/{cnpj}/perfil-completo":      "0.015",
	"/v1/setor/{cnae}/regulacao":               "0.015",
	// $0.020 — premium composite: competição, ESG, litígio, mercado de trabalho
	"/v1/mercado/{cnae}/competicao":            "0.020",
	"/v1/ambiental/empresa/{cnpj}/esg":        "0.020",
	"/v1/litigio/{cnpj}/risco":                "0.020",
	// $0.010 — mercado de trabalho analysis
	"/v1/mercado-trabalho/{uf}/analise":        "0.010",
	// $0.030 — network/influence analysis
	"/v1/rede/{cnpj}/influencia":              "0.030",
	// $0.050 — due diligence
	"/v1/empresas/{cnpj}/duediligence":         "0.050",
	// $0.100 — batch portfolio risk analysis
	"/v1/carteira/risco":                       "0.100",
}

const contextSurcharge = "0.001" // ?format=context adds this to base price

// PriceFor returns the USDC price string for a given route pattern.
// Returns ("", false) if the route is not in the price table.
func PriceFor(routePattern string) (string, bool) {
	price, ok := priceTable[routePattern]
	return price, ok
}

// AddContextPrice adds the context surcharge (+$0.001) to a base price string.
// E.g. "0.001" → "0.002".
func AddContextPrice(basePrice string) string {
	base, err := strconv.ParseFloat(basePrice, 64)
	if err != nil {
		return basePrice
	}
	surcharge, _ := strconv.ParseFloat(contextSurcharge, 64)
	total := base + surcharge
	return fmt.Sprintf("%.3f", total)
}

// USDCToAtomicUnits converts a decimal USDC amount string to its 6-decimal atomic unit string.
// E.g. "0.001" → "1000" (USDC has 6 decimals).
func USDCToAtomicUnits(usdc string) string {
	f, err := strconv.ParseFloat(usdc, 64)
	if err != nil {
		return "1000" // fallback: 0.001 USDC
	}
	// USDC has 6 decimals: multiply by 1_000_000
	atomic := new(big.Float).Mul(big.NewFloat(f), big.NewFloat(1_000_000))
	result, _ := atomic.Int(nil)
	return result.String()
}

// allRoutePatterns returns all known route patterns (for documentation/MCP).
func AllRoutePatterns() []string {
	patterns := make([]string, 0, len(priceTable))
	for p := range priceTable {
		patterns = append(patterns, p)
	}
	return patterns
}

// DefaultPrice returns a sensible default price if a route is not in the table.
const DefaultPrice = "0.001"

// IsPublicPath returns true for paths that must bypass x402 (health, metrics).
func IsPublicPath(path string) bool {
	public := []string{"/health", "/metrics", "/favicon.ico"}
	for _, p := range public {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
