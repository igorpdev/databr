// Package x402 provides payment middleware and pricing for the DataBR API.
// It uses the official Coinbase x402 SDK for facilitator communication and
// V2 payment types with Bazaar discovery extensions.
package x402

import (
	"fmt"
	"math/big"
	"strconv"
)

// priceTable maps Chi route patterns to their USDC prices (as decimal strings).
// Pricing calibrated against Bazaar market data (2026-02-23):
//   P25=$0.005  Median=$0.010  P75=$0.050
var priceTable = map[string]string{
	// $0.003 — basic lookups: company data, BCB, economic indicators, tesouro, legislativo, IPEA, IBGE
	"/v1/empresas/{cnpj}":                       "0.003",
	"/v1/empresas/{cnpj}/socios":                "0.003",
	"/v1/empresas/{cnpj}/simples":               "0.003",
	"/v1/endereco/{cep}":                        "0.003",
	"/v1/bcb/selic":                              "0.003",
	"/v1/bcb/cambio/{moeda}":                    "0.003",
	"/v1/bcb/pix/estatisticas":                  "0.003",
	"/v1/bcb/credito":                           "0.003",
	"/v1/bcb/reservas":                          "0.003",
	"/v1/bcb/taxas-credito":                     "0.003",
	"/v1/bcb/indicadores/{serie}":               "0.003",
	"/v1/bcb/capitais":                          "0.003",
	"/v1/bcb/sml":                               "0.003",
	"/v1/bcb/ifdata":                            "0.003",
	"/v1/bcb/base-monetaria":                    "0.003",
	"/v1/economia/ipca":                         "0.003",
	"/v1/economia/pib":                          "0.003",
	"/v1/economia/focus":                        "0.003",
	"/v1/tesouro/rreo":                          "0.003",
	"/v1/tesouro/entes":                         "0.003",
	"/v1/tesouro/rgf":                           "0.003",
	"/v1/tesouro/dca":                           "0.003",
	"/v1/tesouro/titulos":                       "0.003",
	"/v1/compliance/ceis/{cnpj}":                "0.003",
	"/v1/compliance/cnep/{cnpj}":                "0.003",
	"/v1/compliance/cepim/{cnpj}":               "0.003",
	"/v1/transparencia/contratos":               "0.003",
	"/v1/transparencia/servidores":              "0.003",
	"/v1/transparencia/beneficios":              "0.003",
	"/v1/transparencia/cartoes":                 "0.003",
	"/v1/transparencia/ceaf/{cnpj}":             "0.003",
	"/v1/transparencia/viagens":                 "0.003",
	"/v1/transparencia/emendas":                 "0.003",
	"/v1/transparencia/obras":                   "0.003",
	"/v1/transparencia/transferencias":          "0.003",
	"/v1/transparencia/pensionistas":            "0.003",
	"/v1/transparencia/licitacoes":              "0.003",
	"/v1/ibge/municipio/{ibge}":                 "0.003",
	"/v1/ibge/municipios/{uf}":                  "0.003",
	"/v1/ibge/estados":                          "0.003",
	"/v1/ibge/regioes":                          "0.003",
	"/v1/ibge/cnae/{codigo}":                    "0.003",
	"/v1/ibge/pnad":                             "0.003",
	"/v1/ibge/inpc":                             "0.003",
	"/v1/ibge/pim":                              "0.003",
	"/v1/ibge/populacao":                        "0.003",
	"/v1/ibge/ipca15":                           "0.003",
	"/v1/ibge/pmc":                              "0.003",
	"/v1/ibge/pms":                              "0.003",
	"/v1/legislativo/deputados":                 "0.003",
	"/v1/legislativo/deputados/{id}":            "0.003",
	"/v1/legislativo/deputados/{id}/despesas":   "0.003",
	"/v1/legislativo/proposicoes":               "0.003",
	"/v1/legislativo/votacoes":                  "0.003",
	"/v1/legislativo/partidos":                  "0.003",
	"/v1/legislativo/senado/senadores":          "0.003",
	"/v1/legislativo/senado/materias":           "0.003",
	"/v1/legislativo/eventos":                   "0.003",
	"/v1/legislativo/comissoes":                 "0.003",
	"/v1/legislativo/frentes":                   "0.003",
	"/v1/legislativo/blocos":                    "0.003",
	"/v1/ipea/serie/{codigo}":                   "0.003",
	"/v1/ipea/busca":                            "0.003",
	"/v1/ipea/temas":                            "0.003",
	"/v1/pncp/orgaos":                           "0.003",
	"/v1/eleicoes/candidatos":                   "0.003",
	"/v1/eleicoes/bens":                         "0.003",
	"/v1/eleicoes/doacoes":                      "0.003",
	"/v1/eleicoes/resultados":                   "0.003",
	"/v1/energia/combustiveis":                  "0.003",
	"/v1/energia/tarifas":                       "0.003",
	"/v1/saude/medicamentos/{registro}":         "0.003",
	"/v1/saude/planos":                          "0.003",
	"/v1/transporte/transportadores/{rntrc}":    "0.003",
	"/v1/transporte/aeronaves/{prefixo}":        "0.003",
	"/v1/mercado/fatos-relevantes/{protocolo}":  "0.003",
	// $0.005 — standard: B3 stock quotes, CVM, INPE, transport lists, comex, educacao
	"/v1/mercado/acoes/{ticker}":       "0.005",
	"/v1/mercado/fatos-relevantes":     "0.005",
	"/v1/mercado/fundos/{cnpj}/cotas":  "0.005",
	"/v1/ambiental/desmatamento":       "0.005",
	"/v1/ambiental/prodes":             "0.005",
	"/v1/transporte/aeronaves":         "0.005",
	"/v1/transporte/transportadores":   "0.005",
	"/v1/comercio/exportacoes":         "0.005",
	"/v1/comercio/importacoes":         "0.005",
	"/v1/mercado/indices/ibovespa":     "0.005",
	"/v1/educacao/censo-escolar":       "0.005",
	"/v1/transporte/acidentes":         "0.005",
	"/v1/emprego/rais":                 "0.005",
	"/v1/emprego/caged":                "0.005",
	"/v1/energia/geracao":              "0.005",
	"/v1/energia/carga":                "0.005",
	"/v1/ambiental/uso-solo":           "0.005",
	"/v1/ambiental/embargos":           "0.005",
	// $0.007 — enhanced: compliance via empresa, DOU/diários, environmental risk, electoral compliance
	"/v1/empresas/{cnpj}/compliance":           "0.007",
	"/v1/empresas/{cnpj}/setor":                "0.007",
	"/v1/dou/busca":                            "0.007",
	"/v1/diarios/busca":                        "0.007",
	"/v1/ambiental/risco/{municipio}":          "0.007",
	"/v1/eleicoes/compliance/{cpf_cnpj}":       "0.007",
	"/v1/municipios/{codigo}/perfil":           "0.007",
	// $0.010 — premium: full compliance, CVM funds, judicial superior courts, score, analysis
	"/v1/compliance/{cnpj}":                    "0.010",
	"/v1/mercado/fundos/{cnpj}":                "0.010",
	"/v1/judicial/stf":                         "0.010",
	"/v1/judicial/stj":                         "0.010",
	"/v1/mercado/fundos/{cnpj}/analise":        "0.010",
	"/v1/credito/score/{cnpj}":                 "0.010",
	// $0.015 — advanced: judicial process search, panorama, mercado de trabalho
	"/v1/judicial/processos/{doc}":             "0.015",
	"/v1/economia/panorama":                    "0.015",
	"/v1/mercado-trabalho/{uf}/analise":        "0.015",
	// $0.020 — composite: perfil completo, regulação setorial
	"/v1/empresas/{cnpj}/perfil-completo":      "0.020",
	"/v1/setor/{cnae}/regulacao":               "0.020",
	// $0.030 — deep analysis: competição, ESG, litígio
	"/v1/mercado/{cnae}/competicao":            "0.030",
	"/v1/ambiental/empresa/{cnpj}/esg":        "0.030",
	"/v1/litigio/{cnpj}/risco":                "0.030",
	// $0.050 — network/influence analysis
	"/v1/rede/{cnpj}/influencia":              "0.050",
	// $0.075 — due diligence
	"/v1/empresas/{cnpj}/duediligence":         "0.075",
	// $0.150 — batch portfolio risk analysis
	"/v1/carteira/risco":                       "0.150",
}

const contextSurcharge = "0.002" // ?format=context adds this to base price

// PriceFor returns the USDC price string for a given route pattern.
// Returns ("", false) if the route is not in the price table.
func PriceFor(routePattern string) (string, bool) {
	price, ok := priceTable[routePattern]
	return price, ok
}

// AddContextPrice adds the context surcharge (+$0.002) to a base price string.
// E.g. "0.003" → "0.005".
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

// DefaultPrice is the fallback price when no middleware injects a price (e.g. unit tests).
// Matches the lowest pricing tier ($0.003 basic lookups).
const DefaultPrice = "0.003"

