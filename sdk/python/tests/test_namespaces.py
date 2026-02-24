"""Tests for namespace URL mapping."""

from unittest.mock import MagicMock

from databr.namespaces.bcb import BCBNamespace
from databr.namespaces.empresas import EmpresasNamespace
from databr.namespaces.economia import EconomiaNamespace
from databr.namespaces.mercado import MercadoNamespace
from databr.namespaces.compliance import ComplianceNamespace
from databr.namespaces.judicial import JudicialNamespace
from databr.response import DataBRResponse

MOCK_RAW = {
    "source": "test",
    "updated_at": "2026-01-01T00:00:00Z",
    "cached": False,
    "cache_age_seconds": 0,
    "cost_usdc": "0.003",
    "data": {},
}


def _make_client_mock():
    client = MagicMock()
    client.get.return_value = DataBRResponse.from_dict(MOCK_RAW)
    client.post.return_value = DataBRResponse.from_dict(MOCK_RAW)
    return client


def test_bcb_selic():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.selic()
    client.get.assert_called_once_with("/v1/bcb/selic")


def test_bcb_cambio():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.cambio("USD")
    client.get.assert_called_once_with("/v1/bcb/cambio/USD")


def test_bcb_focus():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.focus()
    client.get.assert_called_once_with("/v1/bcb/focus")


def test_bcb_selic_with_kwargs():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.selic(format="context", fields=["valor"])
    client.get.assert_called_once_with("/v1/bcb/selic", format="context", fields=["valor"])


def test_bcb_credito():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.credito()
    client.get.assert_called_once_with("/v1/bcb/credito")


def test_bcb_reservas():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.reservas()
    client.get.assert_called_once_with("/v1/bcb/reservas")


def test_bcb_taxas_credito():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.taxas_credito()
    client.get.assert_called_once_with("/v1/bcb/taxas-credito")


def test_bcb_pix():
    client = _make_client_mock()
    ns = BCBNamespace(client)
    ns.pix()
    client.get.assert_called_once_with("/v1/bcb/pix/estatisticas")


def test_empresas_consultar():
    client = _make_client_mock()
    ns = EmpresasNamespace(client)
    ns.consultar("12345678000190")
    client.get.assert_called_once_with("/v1/empresas/12345678000190")


def test_empresas_due_diligence():
    client = _make_client_mock()
    ns = EmpresasNamespace(client)
    ns.due_diligence("12345678000190")
    client.get.assert_called_once_with("/v1/empresas/12345678000190/duediligence")


def test_economia_ipca():
    client = _make_client_mock()
    ns = EconomiaNamespace(client)
    ns.ipca()
    client.get.assert_called_once_with("/v1/economia/ipca")


def test_mercado_acoes():
    client = _make_client_mock()
    ns = MercadoNamespace(client)
    ns.acoes("PETR4")
    client.get.assert_called_once_with("/v1/mercado/acoes/PETR4")


def test_compliance_verificar():
    client = _make_client_mock()
    ns = ComplianceNamespace(client)
    ns.verificar("12345678000190")
    client.get.assert_called_once_with("/v1/compliance/12345678000190")


def test_judicial_processos():
    client = _make_client_mock()
    ns = JudicialNamespace(client)
    ns.processos("12345678000190")
    client.get.assert_called_once_with("/v1/judicial/processos/12345678000190")
