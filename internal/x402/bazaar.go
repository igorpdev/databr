package x402

import "strings"

// routeMetaEntry holds human-readable metadata for a single API endpoint.
type routeMetaEntry struct {
	description string
	mimeType    string
}

// routeMeta maps Chi route patterns to metadata surfaced in the x402 Bazaar index.
// Keys must match the full pattern returned by chi.RouteContext.RoutePattern(),
// including any /v1 prefix from r.Route("/v1", ...).
var routeMeta = map[string]routeMetaEntry{
	// Empresas — Receita Federal via minhareceita.org
	"/v1/empresas/{cnpj}":            {"Dados cadastrais de empresa por CNPJ — razão social, CNAE, endereço, situação cadastral (Receita Federal)", "application/json"},
	"/v1/empresas/{cnpj}/compliance": {"Compliance de empresa — cruzamento CEIS, CNEP, CEPIM e processos judiciais por CNPJ (CGU + DataJud)", "application/json"},
	"/v1/empresas/{cnpj}/socios":     {"Quadro societário completo — sócios, qualificação e participação por CNPJ (Receita Federal)", "application/json"},
	"/v1/empresas/{cnpj}/simples":    {"Situação no Simples Nacional e MEI — opção vigente e data de adesão por CNPJ (Receita Federal)", "application/json"},
	// BCB — Banco Central do Brasil
	"/v1/bcb/selic":             {"Taxa Selic — taxa básica de juros da economia brasileira, atualizada a cada reunião do COPOM (Banco Central do Brasil)", "application/json"},
	"/v1/bcb/cambio/{moeda}":   {"Cotação PTAX — taxa de câmbio de referência diária, compra e venda por moeda (Banco Central do Brasil)", "application/json"},
	"/v1/bcb/pix/estatisticas": {"Estatísticas do PIX — volume de transações, valores e quantidade de chaves registradas (Banco Central do Brasil)", "application/json"},
	"/v1/bcb/credito":          {"Operações de crédito do SFN — saldo, inadimplência e concessões por segmento (Banco Central do Brasil)", "application/json"},
	"/v1/bcb/reservas":         {"Reservas internacionais do Brasil — posição diária em USD, composição por moeda (Banco Central do Brasil)", "application/json"},
	"/v1/bcb/taxas-credito":    {"Taxas de juros do crédito — taxas médias pré e pós-fixadas por modalidade (Banco Central do Brasil, OLINDA)", "application/json"},
	"/v1/bcb/indicadores/{serie}": {"Séries históricas do SGS — CDI, IGP-M, IPCA, TR, Dólar, Desemprego e 30.000+ indicadores (Banco Central do Brasil)", "application/json"},
	"/v1/bcb/capitais":         {"IED - Investimento Estrangeiro Direto — registros de capital estrangeiro no país (Banco Central do Brasil, RDE)", "application/json"},
	"/v1/bcb/sml":              {"SML - Sistema de Moeda Local — cotações para pagamentos Brasil-Paraguai/Uruguai/Argentina (Banco Central do Brasil)", "application/json"},
	"/v1/bcb/ifdata":           {"IFDATA — cadastro de instituições financeiras autorizadas pelo Banco Central, ativos e patrimônio", "application/json"},
	"/v1/bcb/base-monetaria":   {"Base monetária — agregados M0 e M2, papel-moeda e depósitos bancários (Banco Central do Brasil, SGS)", "application/json"},
	// Economia — indicadores macroeconômicos
	"/v1/economia/ipca":  {"IPCA - Índice Nacional de Preços ao Consumidor Amplo — variação mensal e acumulado 12 meses (IBGE SIDRA)", "application/json"},
	"/v1/economia/pib":   {"PIB - Produto Interno Bruto — variação trimestral e acumulada, preços correntes e constantes (IBGE SIDRA)", "application/json"},
	"/v1/economia/focus": {"Relatório Focus — expectativas de mercado para Selic, IPCA, PIB e câmbio, atualizado semanalmente (Banco Central do Brasil)", "application/json"},
	// Mercado financeiro — B3, CVM
	"/v1/mercado/acoes/{ticker}":                {"Cotação histórica de ações — preço de fechamento, abertura, máxima, mínima e volume por ticker (B3 BM&FBovespa)", "application/json"},
	"/v1/mercado/fundos/{cnpj}":                 {"Fundo de investimento — informações cadastrais, gestor, administrador e patrimônio líquido por CNPJ (CVM)", "application/json"},
	"/v1/mercado/fundos/{cnpj}/cotas":           {"Histórico de cotas de fundo — valor da cota, captação e resgate diários por CNPJ (CVM)", "application/json"},
	"/v1/mercado/fundos/{cnpj}/analise":         {"Análise de fundo de investimento — rentabilidade, volatilidade e comparativo vs CDI/IPCA (CVM + Banco Central)", "application/json"},
	"/v1/mercado/fatos-relevantes":              {"Fatos relevantes — comunicados de empresas listadas na B3 sobre eventos materiais (CVM)", "application/json"},
	"/v1/mercado/fatos-relevantes/{protocolo}":  {"Fato relevante específico por número de protocolo — texto completo do comunicado (CVM)", "application/json"},
	"/v1/mercado/indices/ibovespa":              {"Composição do IBOVESPA — carteira teórica, peso e participação de cada ação no índice (B3)", "application/json"},
	// Compliance — CGU Portal da Transparência
	"/v1/compliance/{cnpj}":       {"Compliance completa por CNPJ — cruzamento CEIS, CNEP, CEPIM, TCU e processos judiciais (CGU + DataJud CNJ)", "application/json"},
	"/v1/compliance/ceis/{cnpj}":  {"CEIS - Cadastro de Empresas Inidôneas e Suspensas — sanções administrativas por CNPJ (CGU Portal da Transparência)", "application/json"},
	"/v1/compliance/cnep/{cnpj}":  {"CNEP - Cadastro Nacional de Empresas Punidas — penalidades da Lei Anticorrupção por CNPJ (CGU Portal da Transparência)", "application/json"},
	"/v1/compliance/cepim/{cnpj}": {"CEPIM - Cadastro de Entidades Privadas Sem Fins Lucrativos Impedidas — por CNPJ (CGU Portal da Transparência)", "application/json"},
	// Transparência — CGU Portal da Transparência
	"/v1/transparencia/licitacoes":     {"Licitações públicas federais — modalidade, valor e órgão licitante (PNCP - Portal Nacional de Contratações Públicas)", "application/json"},
	"/v1/transparencia/contratos":      {"Contratos públicos federais — fornecedor, valor, vigência e objeto (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/servidores":     {"Servidores públicos federais — cargo, órgão e remuneração por nome ou CPF (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/beneficios":     {"Bolsa Família / Auxílio Brasil — beneficiários e valores por município (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/cartoes":        {"Cartão de Pagamento do Governo Federal — CPGF, gastos com suprimento de fundos por órgão (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/ceaf/{cnpj}":    {"CEAF - Cadastro de Entidades Sem Fins Lucrativos — convênios e transferências por CNPJ (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/viagens":        {"Viagens a serviço — passagens, diárias e trechos de servidores federais (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/emendas":        {"Emendas parlamentares — autor, valor empenhado e pago por ano (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/obras":          {"Imóveis funcionais e obras do governo federal — localização e situação (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/transferencias": {"Convênios e transferências voluntárias — SICONV, valor repassado e contrapartida (CGU Portal da Transparência)", "application/json"},
	"/v1/transparencia/pensionistas":   {"Pensionistas e servidores civis — órgão, cargo e remuneração bruta (CGU Portal da Transparência)", "application/json"},
	// Eleições — TSE Tribunal Superior Eleitoral
	"/v1/eleicoes/candidatos":           {"Candidatos eleitorais — nome, partido, cargo e situação da candidatura (TSE - Tribunal Superior Eleitoral)", "application/json"},
	"/v1/eleicoes/bens":                 {"Bens declarados por candidatos — patrimônio, tipo e valor dos bens (TSE - Tribunal Superior Eleitoral)", "application/json"},
	"/v1/eleicoes/doacoes":              {"Doações eleitorais — doador, valor, CPF/CNPJ e candidato receptor (TSE - Tribunal Superior Eleitoral)", "application/json"},
	"/v1/eleicoes/resultados":           {"Resultados eleitorais — votos por candidato, zona eleitoral e turno (TSE - Tribunal Superior Eleitoral)", "application/json"},
	"/v1/eleicoes/compliance/{cpf_cnpj}": {"Compliance eleitoral — cruzamento doações TSE com sanções CEIS/CNEP e processos judiciais", "application/json"},
	// Tesouro Nacional — SICONFI e Tesouro Direto
	"/v1/tesouro/rreo":    {"RREO - Relatório Resumido de Execução Orçamentária — receitas e despesas por ente federativo (Tesouro Nacional, SICONFI)", "application/json"},
	"/v1/tesouro/entes":   {"Entes federativos — municípios e estados cadastrados no SICONFI com código IBGE (Secretaria do Tesouro Nacional)", "application/json"},
	"/v1/tesouro/rgf":     {"RGF - Relatório de Gestão Fiscal — dívida, pessoal e limites da LRF por UF (Tesouro Nacional, SICONFI)", "application/json"},
	"/v1/tesouro/dca":     {"DCA - Declaração de Contas Anuais — balanço patrimonial e demonstrações contábeis de entes (Tesouro Nacional, SICONFI)", "application/json"},
	"/v1/tesouro/titulos": {"Tesouro Direto — preços, taxas e vencimentos dos títulos públicos federais (Secretaria do Tesouro Nacional)", "application/json"},
	// Judicial — DataJud CNJ, STF, STJ
	"/v1/judicial/processos/{doc}":   {"Processos judiciais por CPF/CNPJ — busca em 91 tribunais brasileiros, classe e assunto (DataJud CNJ)", "application/json"},
	"/v1/judicial/processo/{numero}": {"Processo judicial por número CNJ — andamento, partes e decisões em 91 tribunais (DataJud CNJ)", "application/json"},
	"/v1/judicial/stf":              {"Jurisprudência do STF - Supremo Tribunal Federal — acórdãos, decisões monocráticas e repercussão geral", "application/json"},
	"/v1/judicial/stj":              {"Jurisprudência do STJ - Superior Tribunal de Justiça — acórdãos, súmulas e recursos repetitivos", "application/json"},
	// DOU — Diário Oficial da União
	"/v1/dou/busca": {"DOU - Diário Oficial da União — busca textual em atos, portarias, decretos e nomeações do governo federal", "application/json"},
	// Diários oficiais municipais — Querido Diário (already enriched)
	"/v1/diarios/busca":       {"Busca em diários oficiais municipais (Querido Diário)", "application/json"},
	"/v1/diarios/municipios":  {"Municípios com diários oficiais indexados (Querido Diário, 510+ cidades)", "application/json"},
	"/v1/diarios/temas":       {"Temas de classificação automática de diários oficiais (IA/NLP)", "application/json"},
	"/v1/diarios/tema/{tema}": {"Busca de diários oficiais por tema classificado (ex: Políticas Ambientais)", "application/json"},
	// Saúde — ANVISA, DATASUS, ANS
	"/v1/saude/medicamentos/{registro}":     {"Medicamento por número de registro — princípio ativo, classe terapêutica e validade (ANVISA)", "application/json"},
	"/v1/saude/estabelecimentos/{cnes}":     {"Estabelecimento de saúde por código CNES — tipo, leitos e equipamentos (DATASUS/CNES)", "application/json"},
	"/v1/saude/estabelecimentos":            {"Estabelecimentos de saúde — busca por município ou UF, tipo e natureza jurídica (DATASUS/CNES)", "application/json"},
	"/v1/saude/planos":                      {"Operadoras de planos de saúde — razão social, modalidade e beneficiários ativos (ANS - Agência Nacional de Saúde Suplementar)", "application/json"},
	// DATASUS advanced (already enriched)
	"/v1/saude/mortalidade":     {"Dados de mortalidade do SIM/DATASUS — causas de óbito por CID-10, município, sexo, idade", "application/json"},
	"/v1/saude/nascimentos":     {"Dados de nascidos vivos SINASC/DATASUS — peso, APGAR, tipo de parto, idade da mãe", "application/json"},
	"/v1/saude/hospitais":       {"Hospitais e leitos disponíveis no Brasil (CNES/DATASUS)", "application/json"},
	"/v1/saude/dengue":          {"Casos notificados de dengue e arboviroses (SINAN/DATASUS)", "application/json"},
	"/v1/saude/vacinacao/{ano}": {"Doses de vacinas aplicadas por ano — PNI/DATASUS (2020-2030)", "application/json"},
	// Energia — ANEEL, ANP, ONS
	"/v1/energia/tarifas":       {"Tarifas de energia elétrica — valores homologados por distribuidora e classe de consumo (ANEEL - Agência Nacional de Energia Elétrica)", "application/json"},
	"/v1/energia/combustiveis":  {"Preços de combustíveis — gasolina, diesel e etanol, série histórica semanal (ANP - Agência Nacional do Petróleo via IPEAData)", "application/json"},
	"/v1/energia/geracao":       {"Geração de energia elétrica — produção diária por fonte (hidráulica, eólica, solar, térmica) e subsistema (ONS)", "application/json"},
	"/v1/energia/carga":         {"Carga de energia elétrica — demanda diária por subsistema (SE/CO, S, NE, N) em MWmed (ONS)", "application/json"},
	// Ambiental — INPE, IBAMA, MapBiomas
	"/v1/ambiental/desmatamento": {"DETER - alertas de desmatamento em tempo real na Amazônia Legal, área e município (INPE - Instituto Nacional de Pesquisas Espaciais)", "application/json"},
	"/v1/ambiental/prodes":       {"PRODES - desmatamento anual consolidado na Amazônia, taxa e incremento por estado (INPE - Instituto Nacional de Pesquisas Espaciais)", "application/json"},
	"/v1/ambiental/uso-solo":     {"MapBiomas — classificação de uso e cobertura do solo por bioma, município e ano (Projeto MapBiomas)", "application/json"},
	"/v1/ambiental/embargos":     {"Áreas embargadas por desmatamento ilegal — localização, data e infração (IBAMA - Instituto Brasileiro do Meio Ambiente)", "application/json"},
	// Transporte — ANAC, ANTT, PRF
	"/v1/transporte/aeronaves/{prefixo}":      {"Aeronave por matrícula — modelo, operador, situação de aeronavegabilidade e categoria (ANAC RAB)", "application/json"},
	"/v1/transporte/aeronaves":                {"Aeronaves registradas no RAB — Registro Aeronáutico Brasileiro, busca por operador (ANAC)", "application/json"},
	"/v1/transporte/transportadores/{rntrc}":  {"Transportador rodoviário por RNTRC — razão social, situação e tipo de veículo (ANTT)", "application/json"},
	"/v1/transporte/transportadores":          {"Transportadores rodoviários de carga — busca por CNPJ no RNTRC (ANTT - Agência Nacional de Transportes Terrestres)", "application/json"},
	"/v1/transporte/acidentes":                {"Acidentes em rodovias federais — local, gravidade, causa e vítimas por período (PRF - Polícia Rodoviária Federal)", "application/json"},
	// IBGE — localidades, CNAE e indicadores SIDRA
	"/v1/ibge/municipio/{ibge}": {"Município por código IBGE — nome, UF, mesorregião, área territorial e população estimada (IBGE)", "application/json"},
	"/v1/ibge/municipios/{uf}":  {"Municípios por UF — lista completa com código IBGE, nome e microrregião (IBGE Localidades)", "application/json"},
	"/v1/ibge/estados":          {"Estados brasileiros — código IBGE, sigla, nome e região de todos os 26 estados + DF (IBGE)", "application/json"},
	"/v1/ibge/regioes":          {"Regiões do Brasil — Norte, Nordeste, Centro-Oeste, Sudeste e Sul com estados (IBGE)", "application/json"},
	"/v1/ibge/cnae/{codigo}":    {"CNAE - Classificação Nacional de Atividades Econômicas — subclasse por código com descrição (IBGE)", "application/json"},
	"/v1/ibge/pnad":             {"PNAD Contínua — taxa de desocupação trimestral, população ocupada e força de trabalho (IBGE SIDRA)", "application/json"},
	"/v1/ibge/inpc":             {"INPC - Índice Nacional de Preços ao Consumidor — variação mensal e acumulada, renda 1-5 SM (IBGE SIDRA)", "application/json"},
	"/v1/ibge/pim":              {"PIM-PF - Produção Industrial Mensal — índice de produção física por setor industrial (IBGE SIDRA)", "application/json"},
	"/v1/ibge/populacao":        {"Estimativa populacional — população residente por estado e município, projeções anuais (IBGE SIDRA)", "application/json"},
	"/v1/ibge/ipca15":           {"IPCA-15 — prévia da inflação oficial, variação mensal e grupos de produtos (IBGE SIDRA)", "application/json"},
	"/v1/ibge/pmc":              {"PMC - Pesquisa Mensal do Comércio — volume de vendas do varejo por segmento (IBGE SIDRA)", "application/json"},
	"/v1/ibge/pms":              {"PMS - Pesquisa Mensal de Serviços — receita nominal e volume do setor de serviços (IBGE SIDRA)", "application/json"},
	// Legislativo — Câmara dos Deputados + Senado Federal
	"/v1/legislativo/deputados":                {"Deputados federais em exercício — nome, partido, UF e legislatura atual (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/deputados/{id}":           {"Deputado federal por ID — dados pessoais, gabinete, comissões e situação (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/deputados/{id}/despesas":  {"CEAP - Cota para Exercício da Atividade Parlamentar — despesas detalhadas por deputado (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/proposicoes":              {"Proposições legislativas — PLs, PECs, MPs, tipo, ementa e tramitação (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/votacoes":                 {"Votações do plenário — projetos votados, resultado e orientação de bancada (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/partidos":                 {"Partidos políticos — sigla, nome, líder e número de membros na Câmara (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/senado/senadores":         {"Senadores em exercício — nome, partido, UF e mandato (Senado Federal)", "application/json"},
	"/v1/legislativo/senado/materias":          {"Matérias legislativas — PLPs, PLCs, ementa e situação de tramitação (Senado Federal)", "application/json"},
	"/v1/legislativo/eventos":                  {"Eventos legislativos — audiências públicas, seminários e reuniões com pauta (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/comissoes":                {"Comissões permanentes e especiais — nome, sigla e membros titulares (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/frentes":                  {"Frentes parlamentares — tema, coordenador e deputados participantes (Câmara dos Deputados)", "application/json"},
	"/v1/legislativo/blocos":                   {"Blocos partidários — partidos integrantes e líder do bloco na Câmara (Câmara dos Deputados)", "application/json"},
	// IPEA — Instituto de Pesquisa Econômica Aplicada
	"/v1/ipea/serie/{codigo}": {"Série histórica IPEAData — indicadores econômicos, sociais e regionais com periodicidade variável (IPEA)", "application/json"},
	"/v1/ipea/busca":          {"Busca de séries IPEAData — pesquisa por nome em 9.000+ indicadores econômicos e sociais (IPEA)", "application/json"},
	"/v1/ipea/temas":          {"Temas IPEAData — categorias temáticas (macroeconômico, social, regional, agropecuário) para navegação de séries (IPEA)", "application/json"},
	// PNCP — Portal Nacional de Contratações Públicas
	"/v1/pncp/orgaos": {"Órgãos compradores — entidades cadastradas no PNCP com UF e esfera de governo (Portal Nacional de Contratações Públicas)", "application/json"},
	// CEP — ViaCEP
	"/v1/endereco/{cep}": {"Endereço por CEP — logradouro, bairro, cidade, UF e código IBGE (ViaCEP)", "application/json"},
	// Comércio exterior — MDIC ComexStat
	"/v1/comercio/exportacoes": {"Exportações brasileiras — valor FOB, peso, NCM e país de destino por período (ComexStat MDIC)", "application/json"},
	"/v1/comercio/importacoes": {"Importações brasileiras — valor CIF, peso, NCM e país de origem por período (ComexStat MDIC)", "application/json"},
	// Educação — INEP
	"/v1/educacao/censo-escolar": {"Censo Escolar — matrículas, docentes, escolas e indicadores educacionais por município (INEP - Instituto Nacional de Estudos e Pesquisas)", "application/json"},
	// Emprego — RAIS e CAGED
	"/v1/emprego/rais":  {"RAIS - Relação Anual de Informações Sociais — estoque de empregos formais por setor CNAE e salário médio (Ministério do Trabalho)", "application/json"},
	"/v1/emprego/caged": {"CAGED - Cadastro Geral de Empregados e Desempregados — admissões e desligamentos mensais por setor (Ministério do Trabalho)", "application/json"},
	// Tributário — IBPT e ICMS
	"/v1/tributario/ncm/{codigo}": {"Carga tributária aproximada por NCM/NBS — alíquotas federal, estadual e municipal para produtos e serviços (IBPT De Olho no Imposto)", "application/json"},
	"/v1/tributario/icms/{uf}":    {"Alíquota interna ICMS por estado — taxa modal, FCP e grupo interestadual atualizados para 2026 (CONFAZ)", "application/json"},
	"/v1/tributario/icms":         {"Tabela ICMS completa — alíquotas internas dos 27 estados ou cálculo interestadual com DIFAL (?origem=SP&destino=MA)", "application/json"},
	// Premium cross-referencing endpoints
	"/v1/empresas/{cnpj}/duediligence":    {"Due diligence empresarial completa — CNPJ + CEIS/CNEP + processos judiciais + licitações + ambiental (multi-fonte)", "application/json"},
	"/v1/economia/panorama":               {"Panorama macroeconômico consolidado — Selic, IPCA, PIB, câmbio, Focus e reservas internacionais em tempo real", "application/json"},
	"/v1/empresas/{cnpj}/setor":           {"Análise setorial de empresa — classificação CNAE, porte relativo e contexto do setor (IBGE + B3)", "application/json"},
	"/v1/ambiental/risco/{municipio}":     {"Risco ambiental por município — alertas DETER, desmatamento PRODES e embargos IBAMA consolidados", "application/json"},
	"/v1/credito/score/{cnpj}":            {"Score de crédito público — indicador baseado em compliance, judicial, fiscal e setor de atuação (dados públicos)", "application/json"},
	"/v1/municipios/{codigo}/perfil":      {"Perfil completo de município — demografia IBGE, finanças SICONFI, ambiental e licitações consolidados", "application/json"},
	// Premium composite endpoints
	"/v1/empresas/{cnpj}/perfil-completo": {"Perfil empresarial 360 — CNPJ + sócios + compliance + judicial + contratos + ambiental + setor (multi-fonte)", "application/json"},
	"/v1/carteira/risco":                  {"Análise de risco de carteira em batch — score de até 50 CNPJs com compliance, judicial e crédito", "application/json"},
	"/v1/rede/{cnpj}/influencia":          {"Rede de influência societária — grafo de sócios, empresas conectadas e participações cruzadas", "application/json"},
	"/v1/litigio/{cnpj}/risco":            {"Risco de litígio empresarial — processos ativos, tendência histórica e exposição financeira estimada", "application/json"},
	"/v1/mercado/{cnae}/competicao":       {"Inteligência competitiva setorial — índice HHI, market share e licitações por setor CNAE", "application/json"},
	"/v1/mercado-trabalho/{uf}/analise":   {"Mercado de trabalho por UF — RAIS + CAGED, setores em crescimento e salário médio por CNAE", "application/json"},
	"/v1/setor/{cnae}/regulacao":          {"Panorama regulatório por setor CNAE — agências reguladoras, exigências de compliance e legislação aplicável", "application/json"},
	"/v1/ambiental/empresa/{cnpj}/esg":    {"Score ESG de empresa — indicadores ambientais (embargos, desmatamento), sociais e de governança", "application/json"},
	// TCU — Tribunal de Contas da União
	"/v1/tcu/acordaos":           {"Acórdãos do TCU - Tribunal de Contas da União — deliberações sobre contas, licitações e contratos públicos", "application/json"},
	"/v1/tcu/certidao/{cnpj}":    {"Certidão de licitante do TCU — situação de regularidade de empresa perante o Tribunal de Contas da União", "application/json"},
	"/v1/tcu/inabilitados":       {"Responsáveis inabilitados pelo TCU — pessoas físicas impedidas de exercer cargo ou função pública", "application/json"},
	"/v1/tcu/inabilitados/{cpf}": {"Responsável inabilitado por CPF — situação de inabilitação perante o Tribunal de Contas da União", "application/json"},
	"/v1/tcu/contratos":          {"Contratos fiscalizados pelo TCU — objetos sob deliberação e determinações do Tribunal de Contas da União", "application/json"},
	// Orçamento — SIAFI via CGU
	"/v1/orcamento/despesas":               {"Despesas do orçamento federal — empenho, liquidação e pagamento por órgão e função (SIAFI via CGU)", "application/json"},
	"/v1/orcamento/funcional-programatica": {"Execução funcional-programática — despesas por função, subfunção, programa e ação orçamentária (SIAFI via CGU)", "application/json"},
	"/v1/orcamento/documento/{codigo}":     {"Documento orçamentário por código — nota de empenho, ordem bancária ou documento de arrecadação (SIAFI via CGU)", "application/json"},
	"/v1/orcamento/documentos":             {"Documentos orçamentários por período — notas de empenho e ordens bancárias federais (SIAFI via CGU)", "application/json"},
	"/v1/orcamento/favorecidos":            {"Favorecidos do orçamento federal — CPF/CNPJ, valor recebido e programa orçamentário (SIAFI via CGU)", "application/json"},
	// Transparência Federal (novos endpoints)
	"/v1/transparencia/pgfn":       {"Dívida ativa PGFN — débitos inscritos na Procuradoria-Geral da Fazenda Nacional por CNPJ (Portal da Transparência)", "application/json"},
	"/v1/transparencia/pep":        {"Pessoas Expostas Politicamente (PEP) — cargos públicos relevantes e vínculos políticos por nome (Portal da Transparência)", "application/json"},
	"/v1/transparencia/leniencias": {"Acordos de leniência — contratos firmados com empresas colaboradoras em investigações (CGU via Portal da Transparência)", "application/json"},
	"/v1/transparencia/renuncias":  {"Renúncias fiscais — benefícios tributários, isenções e desonerações por CNPJ (Portal da Transparência)", "application/json"},
	// BNDES
	"/v1/bndes/{cnpj}/operacoes": {"Operações de crédito do BNDES — financiamentos diretos e indiretos por CNPJ (BNDES Dados Abertos CKAN)", "application/json"},
	// TSE Filiados
	"/v1/eleicoes/filiados": {"Filiados partidários por UF — quantitativo de filiados por partido e estado (TSE Estatística Eleitoral)", "application/json"},
}

// RouteMeta returns the description and mimeType for a route pattern, with a fallback.
func RouteMeta(pattern string) (description, mimeType string) {
	if m, ok := routeMeta[pattern]; ok {
		return m.description, m.mimeType
	}
	return "DataBR — dados públicos brasileiros", "application/json"
}

// matchRouteMeta finds the routeMetaEntry for a concrete URL path by matching
// against parameterized patterns (e.g. /v1/bcb/cambio/USD matches /v1/bcb/cambio/{moeda}).
func matchRouteMeta(path string) (routeMetaEntry, bool) {
	_, meta, ok := matchRoutePattern(path)
	return meta, ok
}

// matchRoutePattern matches a concrete URL path to a parameterized route pattern.
// Returns the pattern (e.g. "/v1/mercado/acoes/{ticker}"), its metadata, and whether
// a match was found. Used to build canonical resource URLs for the Bazaar index so that
// concrete paths like /v1/mercado/acoes/VALE3 and /v1/mercado/acoes/PETR4 resolve to
// the same pattern instead of polluting the index with duplicate entries.
func matchRoutePattern(path string) (pattern string, meta routeMetaEntry, ok bool) {
	// Fast path: exact match (non-parameterized routes like /v1/bcb/selic).
	if m, found := routeMeta[path]; found {
		return path, m, true
	}
	pathParts := strings.Split(path, "/")
	for pat, m := range routeMeta {
		patParts := strings.Split(pat, "/")
		if len(patParts) != len(pathParts) {
			continue
		}
		match := true
		for i, pp := range patParts {
			if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
				continue // wildcard segment
			}
			if pp != pathParts[i] {
				match = false
				break
			}
		}
		if match {
			return pat, m, true
		}
	}
	return "", routeMetaEntry{}, false
}
