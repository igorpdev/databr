# DataBR Python SDK

Python SDK for [DataBR](https://databr.api.br) — Brazilian public data API with automatic x402 payments.

## Install

```bash
pip install databr
```

## Quick Start

```python
from databr import DataBR

client = DataBR(private_key="0x...")

# BCB — Taxa Selic
selic = client.bcb.selic()
print(selic.data)  # {"valor": "14.25", ...}

# Empresa por CNPJ
empresa = client.empresas.consultar("33000167000101")
print(empresa.data["razao_social"])

# Compliance check
compliance = client.compliance.verificar("33000167000101")
```

## How It Works

The SDK handles the [x402 payment protocol](https://x402.org) automatically:

1. You call `client.bcb.selic()`
2. The API returns HTTP 402 (Payment Required)
3. The SDK signs a USDC payment on Base network
4. The SDK retries with the payment proof
5. You get the data back

You need USDC on Base network in the wallet corresponding to your private key.

## API Reference

### Low-Level API

```python
# Direct path access
resp = client.get("/v1/bcb/selic")
resp = client.get("/v1/empresas/33000167000101", format="context")

# Query parameters
resp = client.get("/v1/economia/ipca", fields=["valor"], since="2026-01-01")

# POST endpoints
resp = client.post("/v1/carteira/risco", body={"cnpjs": ["33000167000101"]})
```

### Namespaces

| Namespace | Example | Endpoints |
|-----------|---------|-----------|
| `bcb` | `client.bcb.selic()` | selic, cambio, focus, credito, reservas, taxas_credito, pix |
| `empresas` | `client.empresas.consultar(cnpj)` | consultar, socios, simples, compliance, perfil_completo, setor, due_diligence |
| `economia` | `client.economia.ipca()` | ipca, pib, panorama |
| `mercado` | `client.mercado.acoes("PETR4")` | acoes, fundos, cotas, fatos, indices, competicao |
| `compliance` | `client.compliance.verificar(cnpj)` | verificar, ceis, cnep, cepim |
| `judicial` | `client.judicial.processos(doc)` | processos, stf, stj, litigio |
| `legislativo` | `client.legislativo.deputados()` | deputados, senadores, proposicoes, votacoes |
| `ambiental` | `client.ambiental.desmatamento()` | desmatamento, prodes, embargos, uso_solo, esg, risco |
| `transparencia` | `client.transparencia.licitacoes()` | licitacoes, contratos, servidores, orcamento, tcu, dou |
| `saude` | `client.saude.medicamentos(reg)` | medicamentos, operadoras, estabelecimentos |
| `energia` | `client.energia.tarifas()` | tarifas, geracao, carga, combustiveis |
| `transporte` | `client.transporte.aeronaves()` | aeronaves, transportadores, acidentes |
| `educacao` | `client.educacao.censo_escolar()` | censo_escolar |
| `emprego` | `client.emprego.rais()` | rais, caged, mercado_trabalho |
| `comercio` | `client.comercio.exportacoes()` | exportacoes, importacoes |

### Query Parameters

All methods accept these keyword arguments:

| Parameter | Example | Description |
|-----------|---------|-------------|
| `format` | `format="context"` | LLM-ready text (+$0.002) |
| `fields` | `fields=["valor", "data"]` | Field projection |
| `since` | `since="2026-01-01"` | Filter by date (from) |
| `until` | `until="2026-02-01"` | Filter by date (to) |
| `limit` | `limit=10` | Pagination limit |
| `offset` | `offset=20` | Pagination offset |

### Response

```python
resp = client.bcb.selic()
resp.source           # "bcb_sgs"
resp.data             # {"valor": "14.25", ...}
resp.context          # None (or LLM text if format="context")
resp.updated_at       # datetime(2026, 2, 23, ...)
resp.cached           # True
resp.cache_age_seconds # 3600
resp.cost_usdc        # "0.003"
resp.raw              # full JSON dict
```

### Error Handling

```python
from databr import DataBR, NotFoundError, PaymentError, RateLimitError

client = DataBR(private_key="0x...")

try:
    empresa = client.empresas.consultar("00000000000000")
except NotFoundError:
    print("CNPJ not found")
except PaymentError:
    print("Payment failed — check USDC balance")
except RateLimitError as e:
    print(f"Rate limited — retry in {e.retry_after}s")
```

## Networks

| Network | Usage |
|---------|-------|
| `mainnet` (default) | Base mainnet — real USDC payments |
| `testnet` | Base Sepolia — test USDC (free) |

```python
# Testnet for development
client = DataBR(private_key="0x...", network="testnet")
```

## License

MIT
