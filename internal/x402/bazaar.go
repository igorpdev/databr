package x402

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// routeMetaEntry holds human-readable metadata for a single API endpoint.
type routeMetaEntry struct {
	description string
	mimeType    string
}

// routeMeta maps Chi route patterns to metadata surfaced in the x402 Bazaar index.
// Keys must match the full pattern returned by chi.RouteContext.RoutePattern(),
// including any /v1 prefix from r.Route("/v1", ...).
var routeMeta = map[string]routeMetaEntry{
	"/v1/empresas/{cnpj}":            {"Dados completos de empresa por CNPJ", "application/json"},
	"/v1/empresas/{cnpj}/compliance": {"Consulta de conformidade e compliance de empresa", "application/json"},
	"/v1/bcb/selic":                  {"Taxa Selic do Banco Central", "application/json"},
	"/v1/bcb/cambio/{moeda}":         {"Cotação PTAX do Banco Central", "application/json"},
	"/v1/bcb/pix/estatisticas":       {"Estatísticas do sistema PIX", "application/json"},
	"/v1/bcb/credito":                {"Dados de crédito do Banco Central", "application/json"},
	"/v1/bcb/reservas":               {"Reservas internacionais do Brasil", "application/json"},
	"/v1/economia/ipca":              {"Índice de inflação IPCA (IBGE)", "application/json"},
	"/v1/economia/pib":               {"Produto Interno Bruto (IBGE)", "application/json"},
	"/v1/mercado/acoes/{ticker}":     {"Cotação histórica de ações da B3", "application/json"},
	"/v1/mercado/fundos/{cnpj}":      {"Dados de fundos de investimento (CVM)", "application/json"},
	"/v1/compliance/{cnpj}":          {"Verificação completa de compliance por CNPJ", "application/json"},
	"/v1/transparencia/licitacoes":   {"Licitações públicas (PNCP)", "application/json"},
	"/v1/eleicoes/candidatos":        {"Dados de candidatos eleitorais (TSE)", "application/json"},
	"/v1/tesouro/rreo":                  {"Relatório de Execução Orçamentária (Tesouro Nacional)", "application/json"},
	"/v1/dou/busca":                    {"Busca no Diário Oficial da União", "application/json"},
	"/v1/judicial/processos/{doc}":     {"Processos judiciais por CPF/CNPJ (DataJud CNJ)", "application/json"},
	"/v1/empresas/{cnpj}/socios":              {"Quadro societário de empresa por CNPJ", "application/json"},
	"/v1/compliance/ceis/{cnpj}":              {"Sanções CEIS por CNPJ (Portal da Transparência)", "application/json"},
	"/v1/compliance/cnep/{cnpj}":              {"Punições CNEP por CNPJ (Portal da Transparência)", "application/json"},
	"/v1/compliance/cepim/{cnpj}":             {"Impedimentos CEPIM por CNPJ (Portal da Transparência)", "application/json"},
	"/v1/mercado/fatos-relevantes":            {"Fatos relevantes de empresas listadas (CVM)", "application/json"},
	"/v1/mercado/fatos-relevantes/{protocolo}": {"Fato relevante específico por protocolo (CVM)", "application/json"},
	"/v1/diarios/busca":                       {"Busca em diários oficiais municipais (Querido Diário)", "application/json"},
	"/v1/economia/focus":               {"Expectativas de mercado do Relatório Focus (BCB)", "application/json"},
	"/v1/saude/medicamentos/{registro}": {"Medicamentos registrados na ANVISA", "application/json"},
	"/v1/energia/tarifas":              {"Tarifas de energia elétrica homologadas pela ANEEL", "application/json"},
	"/v1/ambiental/desmatamento":       {"Alertas de desmatamento em tempo real (INPE DETER)", "application/json"},
	"/v1/ambiental/prodes":             {"Desmatamento anual consolidado (INPE PRODES)", "application/json"},
	"/v1/transporte/aeronaves/{prefixo}": {"Aeronave por prefixo de matrícula (ANAC RAB)", "application/json"},
	"/v1/transporte/aeronaves":           {"Consulta de aeronaves cadastradas no RAB (ANAC)", "application/json"},
	"/v1/transporte/transportadores/{rntrc}": {"Transportador rodoviário por RNTRC (ANTT)", "application/json"},
	"/v1/transporte/transportadores":         {"Transportadores rodoviários por CNPJ (ANTT RNTRC)", "application/json"},
	// Phase 7: IBGE localidades + CNAE
	"/v1/ibge/municipio/{ibge}": {"Dados de município por código IBGE", "application/json"},
	"/v1/ibge/municipios/{uf}":  {"Lista de municípios por UF (IBGE)", "application/json"},
	"/v1/ibge/estados":          {"Lista de estados brasileiros (IBGE)", "application/json"},
	"/v1/ibge/regioes":          {"Lista de regiões do Brasil (IBGE)", "application/json"},
	"/v1/ibge/cnae/{codigo}":    {"Subclasse CNAE por código (IBGE)", "application/json"},
	// Phase 7: Legislativo (Câmara + Senado)
	"/v1/legislativo/deputados":            {"Lista de deputados federais (Câmara)", "application/json"},
	"/v1/legislativo/deputados/{id}":       {"Deputado federal por ID (Câmara)", "application/json"},
	"/v1/legislativo/proposicoes":          {"Proposições legislativas da Câmara dos Deputados", "application/json"},
	"/v1/legislativo/votacoes":             {"Votações do plenário da Câmara dos Deputados", "application/json"},
	"/v1/legislativo/partidos":             {"Partidos políticos registrados na Câmara", "application/json"},
	"/v1/legislativo/senado/senadores":     {"Lista de senadores em exercício (Senado Federal)", "application/json"},
	"/v1/legislativo/senado/materias":      {"Matérias legislativas do Senado Federal", "application/json"},
	// Phase 7: cartões do governo
	"/v1/transparencia/cartoes": {"Gastos com cartões corporativos do governo federal (CGU)", "application/json"},
	// Phase 8: BCB SGS indicadores, Câmara eventos/comissões, IPEAData
	"/v1/bcb/indicadores/{serie}":  {"Séries históricas do BCB SGS (CDI, IGP-M, Dólar, Desemprego, etc.)", "application/json"},
	"/v1/legislativo/eventos":      {"Eventos e reuniões legislativas na Câmara dos Deputados", "application/json"},
	"/v1/legislativo/comissoes":    {"Comissões permanentes da Câmara dos Deputados", "application/json"},
	"/v1/ipea/serie/{codigo}":      {"Séries históricas do IPEAData (dados econômicos e sociais)", "application/json"},
	// Phase 9: BCB OLINDA RDE+SML, IBGE SIDRA, SICONFI, Câmara extras, IPEA extras, CGU extras, PNCP
	"/v1/bcb/capitais":                          {"Registros de Investimento Estrangeiro Direto (BCB RDE)", "application/json"},
	"/v1/bcb/sml":                               {"Cotações SML Brasil-Paraguai/Uruguai/Argentina (BCB OLINDA)", "application/json"},
	"/v1/ibge/pnad":                             {"Taxa de desocupação PNAD Contínua (IBGE SIDRA)", "application/json"},
	"/v1/ibge/inpc":                             {"Variação mensal do INPC (IBGE SIDRA)", "application/json"},
	"/v1/ibge/pim":                              {"Índice de produção industrial PIM-PF (IBGE SIDRA)", "application/json"},
	"/v1/ibge/populacao":                        {"Estimativa de população por estado (IBGE SIDRA)", "application/json"},
	"/v1/ibge/ipca15":                           {"Variação mensal do IPCA-15 (IBGE SIDRA)", "application/json"},
	"/v1/tesouro/entes":                         {"Lista de municípios e estados no SICONFI (Tesouro Nacional)", "application/json"},
	"/v1/tesouro/rgf":                           {"Relatório de Gestão Fiscal por UF (SICONFI)", "application/json"},
	"/v1/tesouro/dca":                           {"Declaração de Contas Anuais (SICONFI)", "application/json"},
	"/v1/legislativo/frentes":                   {"Frentes parlamentares da Câmara dos Deputados", "application/json"},
	"/v1/legislativo/blocos":                    {"Blocos partidários da Câmara dos Deputados", "application/json"},
	"/v1/legislativo/deputados/{id}/despesas":   {"Despesas de deputado federal por ID (CEAP)", "application/json"},
	"/v1/ipea/busca":                            {"Busca de séries no IPEAData por nome", "application/json"},
	"/v1/ipea/temas":                            {"Temas temáticos do IPEAData", "application/json"},
	"/v1/transparencia/ceaf/{cnpj}":             {"CEAF - entidades sem fins lucrativos por CNPJ (CGU)", "application/json"},
	"/v1/transparencia/viagens":                 {"Viagens de servidores públicos federais (CGU)", "application/json"},
	"/v1/pncp/orgaos":                           {"Órgãos compradores no Portal Nacional de Contratações Públicas", "application/json"},
	// Phase 10: BCB/IBGE extras, TSE extras, ANS, Portal Transparência extras
	"/v1/bcb/ifdata":                            {"Cadastro de instituições financeiras autorizadas pelo BCB (IFDATA)", "application/json"},
	"/v1/bcb/base-monetaria":                    {"Base monetária M0 e M2 do Brasil (BCB SGS)", "application/json"},
	"/v1/ibge/pmc":                              {"Pesquisa Mensal do Comércio varejista (IBGE SIDRA)", "application/json"},
	"/v1/ibge/pms":                              {"Pesquisa Mensal de Serviços — receita nominal (IBGE SIDRA)", "application/json"},
	"/v1/eleicoes/bens":                         {"Bens declarados por candidatos eleitorais (TSE)", "application/json"},
	"/v1/eleicoes/doacoes":                      {"Doações eleitorais recebidas por candidatos (TSE)", "application/json"},
	"/v1/eleicoes/resultados":                   {"Resultados eleitorais por candidato e zona (TSE)", "application/json"},
	"/v1/energia/combustiveis":                  {"Preços históricos de combustíveis no Brasil (ANP via IPEADATA)", "application/json"},
	"/v1/transparencia/emendas":                 {"Emendas parlamentares por ano (Portal da Transparência CGU)", "application/json"},
	"/v1/transparencia/obras":                   {"Imóveis e obras do governo federal (Portal da Transparência CGU)", "application/json"},
	"/v1/transparencia/transferencias":          {"Convênios e transferências voluntárias federais (Portal da Transparência CGU)", "application/json"},
	"/v1/transparencia/pensionistas":            {"Servidores civis federais por órgão (Portal da Transparência CGU)", "application/json"},
	"/v1/saude/planos":                          {"Operadoras de planos de saúde ativas (ANS)", "application/json"},
	// Phase 6: new endpoints
	"/v1/endereco/{cep}":                        {"Endereço completo por CEP (ViaCEP)", "application/json"},
	"/v1/empresas/{cnpj}/simples":               {"Situação no Simples Nacional e MEI por CNPJ", "application/json"},
	"/v1/transparencia/contratos":               {"Contratos públicos por fornecedor (Portal da Transparência)", "application/json"},
	"/v1/transparencia/servidores":              {"Servidores públicos federais por órgão (CGU)", "application/json"},
	"/v1/transparencia/beneficios":              {"Beneficiários do Bolsa Família por município (CGU)", "application/json"},
	"/v1/bcb/taxas-credito":                     {"Taxas de juros do mercado de crédito (BCB OLINDA)", "application/json"},
	"/v1/tesouro/titulos":                       {"Preços e taxas dos títulos do Tesouro Direto", "application/json"},
	"/v1/mercado/fundos/{cnpj}/cotas":           {"Histórico de cotas de fundos de investimento (CVM)", "application/json"},
	// Phase 11: Premium cross-referencing endpoints
	"/v1/empresas/{cnpj}/duediligence":          {"Due diligence completa de empresa (CNPJ + compliance + judicial + licitações)", "application/json"},
	"/v1/economia/panorama":                     {"Panorama econômico consolidado (Selic, IPCA, PIB, câmbio, Focus, reservas)", "application/json"},
	"/v1/empresas/{cnpj}/setor":                 {"Análise setorial de empresa por CNAE (IBGE + B3)", "application/json"},
	"/v1/ambiental/risco/{municipio}":           {"Risco ambiental por município (DETER + PRODES)", "application/json"},
	"/v1/eleicoes/compliance/{cpf_cnpj}":        {"Compliance eleitoral (TSE doações + CEIS/CNEP + processos)", "application/json"},
	"/v1/credito/score/{cnpj}":                  {"Score de crédito público de empresa (dados públicos)", "application/json"},
	"/v1/municipios/{codigo}/perfil":             {"Perfil completo de município (IBGE + SICONFI + ambiental + licitações)", "application/json"},
	"/v1/mercado/fundos/{cnpj}/analise":         {"Análise de fundo de investimento (CVM + performance vs CDI/IPCA)", "application/json"},
	// Phase 11: New data sources
	"/v1/comercio/exportacoes":                  {"Dados de exportação brasileira (ComexStat MDIC)", "application/json"},
	"/v1/comercio/importacoes":                  {"Dados de importação brasileira (ComexStat MDIC)", "application/json"},
	"/v1/mercado/indices/ibovespa":              {"Composição do índice IBOVESPA (B3)", "application/json"},
	"/v1/educacao/censo-escolar":                {"Indicadores educacionais do censo escolar (INEP)", "application/json"},
	"/v1/transporte/acidentes":                  {"Acidentes de trânsito em rodovias federais (PRF)", "application/json"},
	"/v1/emprego/rais":                          {"Dados de emprego formal por setor (RAIS)", "application/json"},
	"/v1/emprego/caged":                         {"Criação e destruição de empregos mensais (CAGED)", "application/json"},
	"/v1/judicial/stf":                          {"Jurisprudência do Supremo Tribunal Federal", "application/json"},
	"/v1/judicial/stj":                          {"Jurisprudência do Superior Tribunal de Justiça", "application/json"},
	"/v1/ambiental/uso-solo":                    {"Classificação de uso e cobertura do solo (MapBiomas)", "application/json"},
	"/v1/ambiental/embargos":                    {"Áreas embargadas pelo IBAMA", "application/json"},
	"/v1/energia/geracao":                       {"Geração de energia elétrica por fonte (ONS)", "application/json"},
	"/v1/energia/carga":                         {"Carga de energia elétrica por subsistema (ONS)", "application/json"},
	// Phase 12: Premium composite endpoints
	"/v1/empresas/{cnpj}/perfil-completo":       {"Perfil empresarial completo (CNPJ + compliance + judicial + contratos + ambiental)", "application/json"},
	"/v1/carteira/risco":                        {"Análise de risco de carteira em batch (até 50 CNPJs)", "application/json"},
	"/v1/rede/{cnpj}/influencia":                {"Rede de influência societária (sócios + empresas conectadas)", "application/json"},
	"/v1/litigio/{cnpj}/risco":                  {"Risco de litígio empresarial (processos + tendência + exposição)", "application/json"},
	"/v1/mercado/{cnae}/competicao":             {"Inteligência competitiva setorial (HHI + empresas + licitações)", "application/json"},
	"/v1/mercado-trabalho/{uf}/analise":         {"Análise do mercado de trabalho por UF (emprego + setores + tendência)", "application/json"},
	"/v1/setor/{cnae}/regulacao":                {"Panorama regulatório por setor CNAE (agências + compliance + legislação)", "application/json"},
	"/v1/ambiental/empresa/{cnpj}/esg":          {"Score ESG de empresa (ambiental + social + governança)", "application/json"},
}

// BazaarMiddleware intercepts HTTP 402 responses and injects an `extensions.bazaar`
// field into the JSON body, making the endpoint discoverable by the x402 Bazaar index.
//
// Register BEFORE the x402 payment middleware in the Chi middleware chain so that
// this wrapper sees the 402 emitted by the payment gate:
//
//	r.Use(x402pkg.BazaarMiddleware())
//	r.Use(optionalX402(cfg, "0.001"))
func BazaarMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bw := &bazaarWriter{ResponseWriter: w}
			next.ServeHTTP(bw, r)

			var pattern string
			if rc := chi.RouteContext(r.Context()); rc != nil {
				pattern = rc.RoutePattern()
			}
			bw.finalize(pattern)
		})
	}
}

// bazaarWriter wraps http.ResponseWriter to buffer 402 responses for extension injection.
// Non-402 responses are written through immediately without buffering.
type bazaarWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

// WriteHeader delays writing 402 status so we can modify the body first.
// All other status codes pass through immediately.
func (bw *bazaarWriter) WriteHeader(code int) {
	bw.status = code
	if code != http.StatusPaymentRequired {
		bw.ResponseWriter.WriteHeader(code)
	}
}

// Write buffers the body when a 402 has been signalled; otherwise passes through.
func (bw *bazaarWriter) Write(b []byte) (int, error) {
	if bw.status == http.StatusPaymentRequired {
		return bw.buf.Write(b)
	}
	return bw.ResponseWriter.Write(b)
}

// finalize writes the (possibly modified) buffered response.
// For 402 responses it injects the bazaar discovery extension into the JSON body.
// For all other responses it is a no-op (already written through).
func (bw *bazaarWriter) finalize(pattern string) {
	if bw.status != http.StatusPaymentRequired {
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bw.buf.Bytes(), &body); err != nil {
		// Body is not valid JSON — write original response unchanged.
		bw.ResponseWriter.WriteHeader(http.StatusPaymentRequired)
		bw.ResponseWriter.Write(bw.buf.Bytes()) //nolint:errcheck
		return
	}

	meta, ok := routeMeta[pattern]
	if !ok {
		meta = routeMetaEntry{"DataBR — dados públicos brasileiros", "application/json"}
	}

	// Inject discovery fields into each accepts item matching the format
	// expected by the CDP facilitator's Bazaar index. The facilitator reads
	// description and mimeType directly from the accepts entry, and reads
	// discoverable/method from outputSchema.input (matching the official
	// @coinbase/x402 extensions/bazaar format).
	if accepts, ok := body["accepts"].([]interface{}); ok {
		for i, item := range accepts {
			if m, ok := item.(map[string]interface{}); ok {
				m["description"] = meta.description
				m["mimeType"] = meta.mimeType
				m["outputSchema"] = map[string]interface{}{
					"input": map[string]interface{}{
						"discoverable": true,
						"method":       "GET",
						"type":         "http",
					},
					"output": map[string]interface{}{
						"type": "object",
					},
				}
				accepts[i] = m
			}
		}
	}

	modified, err := json.Marshal(body)
	if err != nil {
		bw.ResponseWriter.WriteHeader(http.StatusPaymentRequired)
		bw.ResponseWriter.Write(bw.buf.Bytes()) //nolint:errcheck
		return
	}

	bw.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(modified)))
	bw.ResponseWriter.WriteHeader(http.StatusPaymentRequired)
	bw.ResponseWriter.Write(modified) //nolint:errcheck
}
