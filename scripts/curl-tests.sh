#!/usr/bin/env bash
#
# curl-tests.sh — Comprehensive endpoint test for DataBR API
#
# Usage: ./scripts/curl-tests.sh [base_url]
#   base_url defaults to http://localhost:8080
#

set -euo pipefail

BASE="${1:-http://localhost:8080}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

PASS=0
WARN=0
FAIL=0
TOTAL=0

test_endpoint() {
  local method="$1"
  local path="$2"
  local price="$3"
  local url="${BASE}${path}"

  TOTAL=$((TOTAL + 1))

  # Capture HTTP status code and first line of body
  local tmpfile
  tmpfile=$(mktemp)
  local status
  status=$(curl -s -o "$tmpfile" -w "%{http_code}" --max-time 15 "$url" 2>/dev/null || echo "000")
  local body_first_line
  body_first_line=$(head -c 200 "$tmpfile" 2>/dev/null | tr '\n' ' ' || echo "(empty)")
  rm -f "$tmpfile"

  # Color based on status
  local color
  case "$status" in
    200|201) color="$GREEN"; PASS=$((PASS + 1)) ;;
    402)     color="$YELLOW"; WARN=$((WARN + 1)) ;;
    404)     color="$YELLOW"; WARN=$((WARN + 1)) ;;
    502)     color="$YELLOW"; WARN=$((WARN + 1)) ;;
    *)       color="$RED"; FAIL=$((FAIL + 1)) ;;
  esac

  printf "${color}%-6s %-55s [%s] \$%s${RESET}\n" "$method" "$path" "$status" "$price"
  if [[ "$status" != "200" && "$status" != "402" && "$status" != "404" && "$status" != "502" ]]; then
    printf "       ${RED}%s${RESET}\n" "$body_first_line"
  fi
}

echo ""
echo -e "${BOLD}=======================================${RESET}"
echo -e "${BOLD}  DataBR API — Endpoint Test Suite${RESET}"
echo -e "${BOLD}  Target: ${CYAN}${BASE}${RESET}"
echo -e "${BOLD}=======================================${RESET}"
echo ""

# -------------------------------------------------------------------
# Pre-flight: check if API is running
# -------------------------------------------------------------------
echo -e "${BOLD}[Pre-flight] Checking if API is running...${RESET}"
health_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "${BASE}/health" 2>/dev/null || echo "000")
if [[ "$health_status" == "000" ]]; then
  echo -e "${RED}ERROR: API is not reachable at ${BASE}${RESET}"
  echo "Start the API first: go run cmd/api/main.go"
  exit 1
fi
echo -e "${GREEN}API is running (health=${health_status})${RESET}"
echo ""

# -------------------------------------------------------------------
# Health
# -------------------------------------------------------------------
echo -e "${BOLD}--- Health ---${RESET}"
test_endpoint GET "/health" "free"
echo ""

# -------------------------------------------------------------------
# $0.001 tier — On-demand (always available)
# -------------------------------------------------------------------
echo -e "${BOLD}--- \$0.001 tier (on-demand, no DB) ---${RESET}"
test_endpoint GET "/v1/empresas/33000167000101"                  "0.001"
test_endpoint GET "/v1/empresas/33000167000101/socios"           "0.001"
test_endpoint GET "/v1/empresas/33000167000101/simples"          "0.001"
test_endpoint GET "/v1/endereco/01001000"                        "0.001"
test_endpoint GET "/v1/tesouro/rreo?uf=SP&ano=2024&periodo=1"   "0.001"
test_endpoint GET "/v1/compliance/ceis/33000167000101"           "0.001"
test_endpoint GET "/v1/compliance/cnep/33000167000101"           "0.001"
test_endpoint GET "/v1/compliance/cepim/33000167000101"          "0.001"
test_endpoint GET "/v1/transparencia/contratos?orgao=26000"       "0.001"
test_endpoint GET "/v1/transparencia/servidores?orgao=26000"     "0.001"
test_endpoint GET "/v1/transparencia/beneficios?municipio_ibge=3550308&mes=202401" "0.001"
test_endpoint GET "/v1/transparencia/cartoes?orgao=26000"        "0.001"
test_endpoint GET "/v1/ibge/municipio/3550308"                   "0.001"
test_endpoint GET "/v1/ibge/municipios/SP"                       "0.001"
test_endpoint GET "/v1/ibge/estados"                             "0.001"
test_endpoint GET "/v1/ibge/regioes"                             "0.001"
test_endpoint GET "/v1/ibge/cnae/6201501"                        "0.001"
test_endpoint GET "/v1/legislativo/deputados"                    "0.001"
test_endpoint GET "/v1/legislativo/deputados/204554"             "0.001"
test_endpoint GET "/v1/legislativo/proposicoes"                  "0.001"
test_endpoint GET "/v1/legislativo/votacoes"                     "0.001"
test_endpoint GET "/v1/legislativo/partidos"                     "0.001"
test_endpoint GET "/v1/legislativo/senado/senadores"             "0.001"
test_endpoint GET "/v1/legislativo/senado/materias"              "0.001"
test_endpoint GET "/v1/legislativo/eventos"                      "0.001"
test_endpoint GET "/v1/legislativo/comissoes"                    "0.001"
test_endpoint GET "/v1/ipea/serie/BM12_TJOVER12"                "0.001"
test_endpoint GET "/v1/ipea/busca?q=inflacao"                    "0.001"
test_endpoint GET "/v1/ipea/temas"                               "0.001"
test_endpoint GET "/v1/bcb/indicadores/selic"                    "0.001"
test_endpoint GET "/v1/bcb/capitais"                             "0.001"
test_endpoint GET "/v1/bcb/sml"                                  "0.001"
test_endpoint GET "/v1/ibge/pnad"                                "0.001"
test_endpoint GET "/v1/ibge/inpc"                                "0.001"
test_endpoint GET "/v1/ibge/pim"                                 "0.001"
test_endpoint GET "/v1/ibge/populacao"                           "0.001"
test_endpoint GET "/v1/ibge/ipca15"                              "0.001"
test_endpoint GET "/v1/tesouro/entes"                            "0.001"
test_endpoint GET "/v1/tesouro/rgf?uf=SP&ano=2024&periodo=1"      "0.001"
test_endpoint GET "/v1/tesouro/dca"                              "0.001"
test_endpoint GET "/v1/legislativo/frentes"                      "0.001"
test_endpoint GET "/v1/legislativo/blocos"                       "0.001"
test_endpoint GET "/v1/legislativo/deputados/204554/despesas"    "0.001"
test_endpoint GET "/v1/transparencia/ceaf/33000167000101"        "0.001"
test_endpoint GET "/v1/transparencia/viagens?orgao=26000"         "0.001"
test_endpoint GET "/v1/transparencia/emendas?ano=2024"           "0.001"
test_endpoint GET "/v1/transparencia/obras?uf=SP"                "0.001"
test_endpoint GET "/v1/transparencia/transferencias?orgao=26000" "0.001"
test_endpoint GET "/v1/transparencia/pensionistas?orgao=26000"   "0.001"
test_endpoint GET "/v1/pncp/orgaos"                              "0.001"
test_endpoint GET "/v1/bcb/ifdata"                               "0.001"
test_endpoint GET "/v1/bcb/base-monetaria"                       "0.001"
test_endpoint GET "/v1/ibge/pmc"                                 "0.001"
test_endpoint GET "/v1/ibge/pms"                                 "0.001"
test_endpoint GET "/v1/eleicoes/bens?ano=2024"                    "0.001"
test_endpoint GET "/v1/eleicoes/doacoes?ano=2024"                "0.001"
test_endpoint GET "/v1/eleicoes/resultados?ano=2024"             "0.001"
test_endpoint GET "/v1/energia/combustiveis"                     "0.001"
test_endpoint GET "/v1/saude/planos"                             "0.001"
echo ""

# -------------------------------------------------------------------
# $0.001 tier — Store-backed (require DB)
# -------------------------------------------------------------------
echo -e "${BOLD}--- \$0.001 tier (store-backed, require DB) ---${RESET}"
test_endpoint GET "/v1/bcb/selic"                                "0.001"
test_endpoint GET "/v1/bcb/cambio/USD"                           "0.001"
test_endpoint GET "/v1/bcb/pix/estatisticas"                     "0.001"
test_endpoint GET "/v1/bcb/credito"                              "0.001"
test_endpoint GET "/v1/bcb/reservas"                             "0.001"
test_endpoint GET "/v1/bcb/taxas-credito"                        "0.001"
test_endpoint GET "/v1/tesouro/titulos"                          "0.001"
test_endpoint GET "/v1/economia/ipca"                            "0.001"
test_endpoint GET "/v1/economia/pib"                             "0.001"
test_endpoint GET "/v1/economia/focus"                           "0.001"
test_endpoint GET "/v1/transparencia/licitacoes"                 "0.001"
test_endpoint GET "/v1/eleicoes/candidatos"                      "0.001"
test_endpoint GET "/v1/saude/medicamentos/100650001"             "0.001"
test_endpoint GET "/v1/energia/tarifas"                          "0.001"
test_endpoint GET "/v1/mercado/fatos-relevantes/12345"           "0.001"
test_endpoint GET "/v1/transporte/aeronaves/PTUHB"               "0.001"
test_endpoint GET "/v1/transporte/transportadores/12345678"      "0.001"
echo ""

# -------------------------------------------------------------------
# $0.002 tier
# -------------------------------------------------------------------
echo -e "${BOLD}--- \$0.002 tier ---${RESET}"
test_endpoint GET "/v1/mercado/acoes/PETR4"                      "0.002"
test_endpoint GET "/v1/mercado/fatos-relevantes"                 "0.002"
test_endpoint GET "/v1/mercado/fundos/00017024000153/cotas"      "0.002"
test_endpoint GET "/v1/ambiental/desmatamento"                   "0.002"
test_endpoint GET "/v1/ambiental/prodes"                         "0.002"
test_endpoint GET "/v1/transporte/aeronaves"                     "0.002"
test_endpoint GET "/v1/transporte/transportadores?cnpj=33000167000101" "0.002"
echo ""

# -------------------------------------------------------------------
# $0.003 tier
# -------------------------------------------------------------------
echo -e "${BOLD}--- \$0.003 tier ---${RESET}"
test_endpoint GET "/v1/empresas/33000167000101/compliance"       "0.003"
test_endpoint GET "/v1/dou/busca?q=licitacao"                    "0.003"
test_endpoint GET "/v1/diarios/busca?q=licitacao"                "0.003"
echo ""

# -------------------------------------------------------------------
# $0.005 tier
# -------------------------------------------------------------------
echo -e "${BOLD}--- \$0.005 tier ---${RESET}"
test_endpoint GET "/v1/compliance/33000167000101"                "0.005"
test_endpoint GET "/v1/mercado/fundos/00017024000153"            "0.005"
echo ""

# -------------------------------------------------------------------
# $0.010 tier
# -------------------------------------------------------------------
echo -e "${BOLD}--- \$0.010 tier ---${RESET}"
test_endpoint GET "/v1/judicial/processos/12345678901"           "0.010"
echo ""

# -------------------------------------------------------------------
# Premium new endpoints (if registered)
# -------------------------------------------------------------------
echo -e "${BOLD}--- Premium endpoints (new) ---${RESET}"
test_endpoint GET "/v1/empresas/33000167000101/duediligence"     "0.015"
test_endpoint GET "/v1/economia/panorama"                        "0.008"
test_endpoint GET "/v1/empresas/33000167000101/setor"            "0.008"
test_endpoint GET "/v1/ambiental/risco/1500602"                  "0.008"
test_endpoint GET "/v1/eleicoes/compliance/12345678901"          "0.008"
test_endpoint GET "/v1/credito/score/33000167000101"             "0.010"
test_endpoint GET "/v1/municipios/3550308/perfil"                "0.008"
test_endpoint GET "/v1/mercado/fundos/00017024000153/analise"    "0.008"
echo ""

# -------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------
echo -e "${BOLD}=======================================${RESET}"
echo -e "${BOLD}  Test Summary${RESET}"
echo -e "${BOLD}=======================================${RESET}"
echo -e "  Total:   ${BOLD}${TOTAL}${RESET}"
echo -e "  ${GREEN}Pass (200):  ${PASS}${RESET}"
echo -e "  ${YELLOW}Warn (402/404/502): ${WARN}${RESET}"
echo -e "  ${RED}Fail (other): ${FAIL}${RESET}"
echo ""

if [[ "$FAIL" -gt 0 ]]; then
  echo -e "${RED}Some endpoints returned unexpected errors.${RESET}"
  exit 1
else
  echo -e "${GREEN}All endpoints operational (200/402/404/502 upstream).${RESET}"
  exit 0
fi
