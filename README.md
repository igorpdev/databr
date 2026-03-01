# DataBR

API de dados públicos brasileiros para agentes de IA, com pagamento on-chain via protocolo [x402](https://x402.org) (USDC na rede Base).

**Sem cadastro. Sem API key. Pague por consulta.**

[Documentação](https://databr.api.br/docs) | [Python SDK](https://pypi.org/project/databr/) | [MCP Server](https://databr.api.br/.well-known/mcp.json) | [MCP Marketplace](https://mcp-marketplace.io/server/io-github-igorpdev-databr) | [Smithery](https://smithery.ai/servers/databr/databr)

---

## Como funciona

```
Agente IA → GET /v1/bcb/selic → 402 Payment Required
         → Paga $0.003 USDC on-chain (Base)
         → Retry com header X-PAYMENT
         → 200 OK + dados
```

O protocolo x402 permite que agentes de IA paguem por dados automaticamente, sem cadastro ou chave de API. Basta ter USDC na rede Base.

---

## Início rápido

### curl (explorar sem pagar)

```bash
# Health check
curl https://databr.api.br/health

# Ver requisitos de pagamento de qualquer endpoint
curl -I https://databr.api.br/v1/bcb/selic
# → 402 Payment Required + JSON com instruções x402
```

### Python SDK

```bash
pip install databr
```

```python
from databr import DataBR

client = DataBR(private_key="0x...")  # wallet com USDC na Base

# Taxa Selic
selic = client.bcb.selic()
print(selic.data)  # {"valor": "14.25", ...}

# Empresa por CNPJ
empresa = client.empresas.consultar("33000167000101")
print(empresa.data["razao_social"])

# Due diligence completa
dd = client.empresas.due_diligence("33000167000101")

# Formato LLM-ready
ipca = client.economia.ipca(format="context")
print(ipca.context)  # texto pronto para prompt
```

### MCP (Claude, Cursor, etc.)

Listado no [MCP Marketplace](https://mcp-marketplace.io/server/io-github-igorpdev-databr) — instale direto pelo marketplace ou adicione manualmente ao seu MCP client:

```json
{
  "mcpServers": {
    "databr": {
      "url": "https://databr.api.br/mcp"
    }
  }
}
```

170+ ferramentas disponíveis cobrindo todos os endpoints da API.

---

## Endpoints

122 endpoints organizados por domínio. Documentação completa em [databr.api.br/docs](https://databr.api.br/docs).

### Economia

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/bcb/selic` | $0.003 | Taxa Selic vigente |
| `GET /v1/bcb/cambio/{moeda}` | $0.003 | Câmbio PTAX (USD, EUR, etc.) |
| `GET /v1/bcb/credito` | $0.003 | Oferta de crédito |
| `GET /v1/bcb/reservas` | $0.003 | Reservas internacionais |
| `GET /v1/bcb/taxas-credito` | $0.003 | Taxas de juros por modalidade |
| `GET /v1/bcb/pix/estatisticas` | $0.003 | Estatísticas PIX |
| `GET /v1/bcb/focus` | $0.003 | Expectativas Focus (mercado) |
| `GET /v1/economia/ipca` | $0.003 | Inflação IPCA |
| `GET /v1/economia/pib` | $0.003 | PIB trimestral |
| `GET /v1/economia/panorama` | $0.015 | Panorama econômico consolidado |
| `GET /v1/ipea/serie/{codigo}` | $0.003 | Séries IPEA (macro, social, regional) |

### Empresas

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/empresas/{cnpj}` | $0.003 | Dados cadastrais (Receita Federal) |
| `GET /v1/empresas/{cnpj}/compliance` | $0.007 | Check rápido CEIS/CNEP/CEPIM |
| `GET /v1/empresas/{cnpj}/setor` | $0.007 | Análise setorial |
| `GET /v1/empresas/{cnpj}/perfil-completo` | $0.020 | Perfil completo com cruzamentos |
| `GET /v1/empresas/{cnpj}/duediligence` | $0.075 | Due diligence automatizada |
| `GET /v1/compliance/{cnpj}` | $0.010 | Compliance completo (CGU + CNJ) |
| `GET /v1/credito/score/{cnpj}` | $0.010 | Score de crédito estimado |

### Mercado financeiro

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/mercado/acoes/{ticker}` | $0.005 | Cotação de ações (B3) |
| `GET /v1/mercado/indices/ibovespa` | $0.005 | Índice Ibovespa |
| `GET /v1/mercado/fundos/{cnpj}` | $0.010 | Detalhes de fundo (CVM) |
| `GET /v1/mercado/fundos/{cnpj}/cotas` | $0.005 | Cotas de fundo |
| `GET /v1/mercado/fundos/{cnpj}/analise` | $0.010 | Análise de fundo |
| `GET /v1/mercado/fatos-relevantes` | $0.005 | Fatos relevantes CVM |
| `GET /v1/mercado/{cnae}/competicao` | $0.030 | Análise de competição |
| `GET /v1/tesouro/titulos` | $0.003 | Tesouro Direto |
| `GET /v1/bndes/{cnpj}/operacoes` | $0.005 | Operações de crédito BNDES |

### Transparência e governo

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/transparencia/licitacoes` | $0.003 | Licitações PNCP |
| `GET /v1/transparencia/contratos` | $0.003 | Contratos federais |
| `GET /v1/transparencia/servidores` | $0.003 | Servidores públicos |
| `GET /v1/transparencia/pgfn` | $0.003 | Dívida ativa PGFN por CNPJ |
| `GET /v1/transparencia/pep` | $0.003 | Pessoas Expostas Politicamente (por nome) |
| `GET /v1/transparencia/leniencias` | $0.003 | Acordos de leniência por CNPJ |
| `GET /v1/transparencia/renuncias` | $0.003 | Renúncias fiscais por exercício |
| `GET /v1/tcu/acordaos` | $0.003 | Acórdãos do TCU |
| `GET /v1/tcu/certidao/{cnpj}` | $0.003 | Certidão TCU |
| `GET /v1/orcamento/despesas` | $0.003 | Execução orçamentária |
| `GET /v1/dou/busca` | $0.007 | Busca em diários oficiais |

### Legislativo

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/legislativo/deputados` | $0.003 | Deputados federais |
| `GET /v1/legislativo/senado/senadores` | $0.003 | Senadores |
| `GET /v1/legislativo/proposicoes` | $0.003 | Projetos de lei |
| `GET /v1/legislativo/votacoes` | $0.003 | Votações |
| `GET /v1/eleicoes/candidatos` | $0.003 | Candidatos TSE |
| `GET /v1/eleicoes/filiados` | $0.003 | Filiados partidários por UF |
| `GET /v1/eleicoes/compliance/{cpf_cnpj}` | $0.007 | Compliance eleitoral |

### Jurídico

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/judicial/stf` | $0.010 | Decisões do STF |
| `GET /v1/judicial/stj` | $0.010 | Decisões do STJ |
| `GET /v1/judicial/processos/{doc}` | $0.015 | Processos por CPF/CNPJ |
| `GET /v1/litigio/{cnpj}/risco` | $0.030 | Risco litigioso |

### Ambiental

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/ambiental/desmatamento` | $0.005 | Alertas DETER (INPE) |
| `GET /v1/ambiental/prodes` | $0.005 | Monitoramento PRODES |
| `GET /v1/ambiental/embargos` | $0.005 | Embargos IBAMA |
| `GET /v1/ambiental/uso-solo` | $0.005 | Cobertura MapBiomas |
| `GET /v1/ambiental/risco/{municipio}` | $0.007 | Risco ambiental |
| `GET /v1/ambiental/empresa/{cnpj}/esg` | $0.030 | Análise ESG |

### Saúde, energia, transporte e mais

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/saude/medicamentos/{registro}` | $0.003 | Medicamentos ANVISA |
| `GET /v1/saude/planos` | $0.003 | Planos ANS |
| `GET /v1/energia/geracao` | $0.005 | Geração de energia (ONS) |
| `GET /v1/energia/tarifas` | $0.003 | Tarifas ANEEL |
| `GET /v1/energia/combustiveis` | $0.003 | Preços ANP |
| `GET /v1/transporte/aeronaves` | $0.005 | Aeronaves ANAC |
| `GET /v1/transporte/acidentes` | $0.005 | Acidentes PRF |
| `GET /v1/comercio/exportacoes` | $0.005 | Exportações (ComexStat) |
| `GET /v1/educacao/censo-escolar` | $0.005 | Censo escolar INEP |
| `GET /v1/emprego/rais` | $0.005 | RAIS (emprego formal) |
| `GET /v1/emprego/caged` | $0.005 | CAGED (admissões/demissões) |

### Tributário

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/tributario/ncm/{codigo}` | $0.003 | Carga tributária por NCM/NBS (IBPT) |
| `GET /v1/tributario/icms/{uf}` | $0.003 | Alíquota ICMS interna de um estado |
| `GET /v1/tributario/icms` | $0.003 | Tabela ICMS completa (27 UFs) ou interestadual |

### Análises compostas

| Endpoint | Preço | Descrição |
|----------|-------|-----------|
| `GET /v1/rede/{cnpj}/influencia` | $0.050 | Análise de rede e influência |
| `POST /v1/carteira/risco` | $0.150 | Risco de carteira (múltiplos CNPJs) |

---

## Query parameters

Todos os endpoints aceitam:

| Parâmetro | Exemplo | Descrição |
|-----------|---------|-----------|
| `format` | `?format=context` | Texto LLM-ready em vez de JSON (+$0.002) |
| `fields` | `?fields=valor,data` | Projeção de campos |
| `since` | `?since=2026-01-01` | Filtrar a partir de data |
| `until` | `?until=2026-02-01` | Filtrar até data |
| `limit` | `?limit=10` | Limite de registros |
| `offset` | `?offset=20` | Offset para paginação |

---

## Formato de resposta

```json
{
  "source": "bcb_sgs",
  "updated_at": "2026-02-21T10:00:00Z",
  "cached": true,
  "cache_age_seconds": 3600,
  "cost_usdc": "0.003",
  "data": {
    "valor": "14.25",
    "data": "2026-02-21"
  }
}
```

---

## Preços

| Faixa | Preço (USDC) | Exemplos |
|-------|-------------|----------|
| Consultas básicas | $0.003 | BCB, IBGE, empresas, legislativo, tributário |
| Consultas padrão | $0.005 | Ações B3, CVM, comércio exterior |
| Consultas avançadas | $0.007 | Compliance empresa, DOU, risco ambiental |
| Premium | $0.010 | Compliance completo, fundos, decisões STF/STJ |
| Análises cruzadas | $0.015–$0.030 | Panorama, ESG, litígio, competição |
| Análises profundas | $0.050–$0.150 | Rede de influência, due diligence, carteira |
| Formato contexto | +$0.002 | Adicional sobre qualquer endpoint |

Rate limits: **100 req/min** por IP | **500 req/min** por wallet x402.

---

## Python SDK

```bash
pip install databr
```

### Namespaces disponíveis

| Namespace | Exemplo | Métodos |
|-----------|---------|---------|
| `bcb` | `client.bcb.selic()` | selic, cambio, focus, credito, reservas, taxas_credito, pix |
| `empresas` | `client.empresas.consultar(cnpj)` | consultar, compliance, perfil_completo, due_diligence |
| `economia` | `client.economia.ipca()` | ipca, pib, panorama |
| `mercado` | `client.mercado.acoes("PETR4")` | acoes, fundos, cotas, fatos, indices, competicao |
| `compliance` | `client.compliance.verificar(cnpj)` | verificar, ceis, cnep, cepim |
| `judicial` | `client.judicial.processos(doc)` | processos, stf, stj, litigio |
| `legislativo` | `client.legislativo.deputados()` | deputados, senadores, proposicoes, votacoes |
| `ambiental` | `client.ambiental.desmatamento()` | desmatamento, prodes, embargos, uso_solo, esg |
| `transparencia` | `client.transparencia.licitacoes()` | licitacoes, contratos, servidores, orcamento |
| `saude` | `client.saude.medicamentos(reg)` | medicamentos, operadoras, estabelecimentos |
| `energia` | `client.energia.tarifas()` | tarifas, geracao, carga, combustiveis |
| `transporte` | `client.transporte.aeronaves()` | aeronaves, transportadores, acidentes |
| `comercio` | `client.comercio.exportacoes()` | exportacoes, importacoes |
| `emprego` | `client.emprego.rais()` | rais, caged |
| `tributario` | `client.tributario.ncm("22030000", uf="SP")` | ncm, icms |

### Tratamento de erros

```python
from databr import DataBR, NotFoundError, PaymentError, RateLimitError

try:
    empresa = client.empresas.consultar("00000000000000")
except NotFoundError:
    print("CNPJ não encontrado")
except PaymentError:
    print("Pagamento falhou — verificar saldo USDC")
except RateLimitError as e:
    print(f"Rate limit — retry em {e.retry_after}s")
```

### Testnet

```python
client = DataBR(private_key="0x...", network="testnet")  # Base Sepolia, USDC de teste
```

---

## Fontes de dados

42 coletores automáticos + 8 on-demand, cobrindo:

| Fonte | Órgão | Atualização |
|-------|-------|-------------|
| Selic, PTAX, crédito, reservas, PIX, Focus | BCB | Diária/semanal |
| IPCA, PIB, população, pesquisas | IBGE | Diária/mensal |
| Cotações, Ibovespa | B3 | Diária (dias úteis) |
| Fundos, fatos relevantes, cotas | CVM | Diária |
| Candidatos, bens, doações, filiados | TSE | Anual / On-demand |
| Licitações PNCP | Portal da Transparência | Diária |
| Acórdãos, certidões, inabilitados | TCU | Semanal |
| Deputados, senadores, proposições | Câmara/Senado | Diária |
| RREO, RGF, Tesouro Direto | Tesouro Nacional | Diária/mensal |
| Exportações, importações | ComexStat | Mensal |
| Tarifas, geração, carga | ANEEL/ONS | Semanal/diária |
| Medicamentos | ANVISA | Mensal |
| Operadoras, planos | ANS | Semanal |
| DETER, PRODES, MapBiomas | INPE | Diária/mensal |
| Embargos | IBAMA | Semanal |
| Aeronaves, transportadores, acidentes | ANAC/ANTT/PRF | Semanal/mensal |
| Censo escolar | INEP | Semestral |
| RAIS, CAGED | MTE (FTP) | Mensal/anual |
| Decisões STF, STJ | Tribunais superiores | Diária |
| Combustíveis | ANP | Semanal |
| Séries macro | IPEA | Diária |
| Diários oficiais | Querido Diário | On-demand |
| CNPJ | minhareceita.org | On-demand |
| Compliance CEIS/CNEP | CGU | On-demand |
| Processos judiciais | DataJud CNJ | On-demand |
| Carga tributária NCM/NBS | IBPT | On-demand |
| Acordos leniência, PEP, renúncias, PGFN | CGU/Portal da Transparência | On-demand |
| Operações de crédito | BNDES (CKAN) | On-demand |
| Alíquotas ICMS (27 UFs) | CONFAZ/SEFAZ | Estático (2026) |

---

## Desenvolvimento local

### Pré-requisitos

- Go 1.24+
- Docker (PostgreSQL + Redis)

### Setup

```bash
git clone https://github.com/igorpdev/databr.git
cd databr

# Subir PostgreSQL e Redis
docker-compose up -d

# Configurar variáveis
cp .env.example .env
# Editar .env com suas configurações

# Rodar API
go run cmd/api/main.go

# Rodar coletores (em outro terminal)
go run cmd/collector/main.go

# Testes
go test ./...
```

A API sobe em `http://localhost:8080`. Sem `WALLET_ADDRESS` configurado, os endpoints `/v1/*` funcionam sem pagamento (modo dev).

### Estrutura

```
databr/
├── cmd/
│   ├── api/              # Entrypoint REST API + MCP
│   └── collector/        # Scheduler de coletores
├── internal/
│   ├── handlers/         # 30+ handlers HTTP (122 endpoints)
│   ├── collectors/       # 23 pacotes de coletores (50 fontes)
│   ├── repositories/     # PostgreSQL (pgx/v5)
│   ├── cache/            # Redis + cache em memória
│   ├── x402/             # Middleware de pagamento + pricing
│   ├── mcp/              # MCP Server (170+ tools)
│   ├── domain/           # Entidades e interfaces
│   ├── metrics/          # Prometheus
│   └── logging/          # Structured logging (slog)
├── sdk/python/           # Python SDK (PyPI: databr)
├── docs/
│   ├── openapi.yaml      # Especificação OpenAPI 3.0
│   └── landing.html      # Landing page
├── migrations/           # Schema PostgreSQL
├── Dockerfile            # API (multi-stage, ~23MB)
├── Dockerfile.collector  # Coletores
└── docker-compose.yml    # Dev: PostgreSQL 16 + Redis 7
```

---

## Stack

| Camada | Tecnologia |
|--------|-----------|
| Backend | Go 1.24, Chi Router |
| Banco | PostgreSQL 16 (pgx/v5) |
| Cache | Redis (Upstash em produção) |
| Pagamentos | x402 protocol, USDC, Base network |
| Deploy | Railway |
| Métricas | Prometheus |
| MCP | mcp-go (Mark3Labs) |
| SDK | Python (PyPI) |

---

## Variáveis de ambiente

```bash
# Obrigatórias
DATABASE_URL=postgres://databr:databr@localhost:5432/databr
REDIS_URL=redis://localhost:6379

# x402 (testnet)
WALLET_ADDRESS=0x...
X402_NETWORK=base-sepolia
X402_FACILITATOR_URL=https://facilitator.x402.rs

# x402 (mainnet)
# X402_NETWORK=base
# X402_FACILITATOR_URL=https://api.cdp.coinbase.com/platform/v2/x402
# CDP_KEY_ID=...
# CDP_KEY_SECRET=...

# APIs externas (opcionais)
TRANSPARENCIA_API_KEY=    # Portal da Transparência (CGU)
DATAJUD_API_KEY=          # DataJud (CNJ)
MINHARECEITA_URL=https://minhareceita.org
```

---

## Links

- **API**: https://databr.api.br
- **Docs (Scalar)**: https://databr.api.br/docs
- **OpenAPI spec**: https://databr.api.br/openapi.yaml
- **MCP discovery**: https://databr.api.br/.well-known/mcp.json
- **x402 discovery**: https://databr.api.br/.well-known/x402
- **Python SDK**: https://pypi.org/project/databr/
- **Health**: https://databr.api.br/health

---

## Licença

MIT
