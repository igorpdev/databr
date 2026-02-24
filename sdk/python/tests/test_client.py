"""Tests for DataBR client -- x402 payment flow."""

from unittest.mock import patch

import pytest
import requests
import responses

from databr.client import DataBR
from databr.exceptions import NotFoundError, RateLimitError, APIError


# Test private key (DO NOT use in production -- this is a well-known test key)
TEST_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"


def _make_test_client(**kwargs):
    """Create a DataBR client with x402 mocked out (plain requests session).

    The x402_requests function wraps a Session with an HTTPAdapter that intercepts
    402 responses. For unit tests we replace it with a plain Session so the
    ``responses`` mock library can intercept at the urllib3 level without interference.
    """
    with patch("databr.client.x402_requests") as mock_x402:
        mock_x402.return_value = requests.Session()
        return DataBR(private_key=TEST_PRIVATE_KEY, network="testnet", **kwargs)


@responses.activate
def test_get_200_without_payment():
    """If server returns 200 directly, no payment needed."""
    responses.get(
        "https://databr.api.br/v1/bcb/selic",
        json={
            "source": "bcb_sgs",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": True,
            "cache_age_seconds": 3600,
            "cost_usdc": "0.003",
            "data": {"valor": "14.25"},
        },
        status=200,
    )

    client = _make_test_client()
    resp = client.get("/v1/bcb/selic")

    assert resp.source == "bcb_sgs"
    assert resp.data == {"valor": "14.25"}
    assert resp.cost_usdc == "0.003"


@responses.activate
def test_get_404_raises_not_found():
    responses.get(
        "https://databr.api.br/v1/empresas/00000000000000",
        json={"error": "not found"},
        status=404,
    )

    client = _make_test_client()
    with pytest.raises(NotFoundError):
        client.get("/v1/empresas/00000000000000")


@responses.activate
def test_get_429_raises_rate_limit():
    responses.get(
        "https://databr.api.br/v1/bcb/selic",
        json={"error": "rate limit exceeded"},
        status=429,
        headers={"Retry-After": "60"},
    )

    client = _make_test_client()
    with pytest.raises(RateLimitError) as exc_info:
        client.get("/v1/bcb/selic")
    assert exc_info.value.retry_after == 60


@responses.activate
def test_get_500_raises_api_error():
    responses.get(
        "https://databr.api.br/v1/bcb/selic",
        json={"error": "internal server error"},
        status=500,
    )

    client = _make_test_client()
    with pytest.raises(APIError):
        client.get("/v1/bcb/selic")


@responses.activate
def test_query_params_format_context():
    responses.get(
        "https://databr.api.br/v1/bcb/selic",
        json={
            "source": "bcb_sgs",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": False,
            "cache_age_seconds": 0,
            "cost_usdc": "0.005",
            "context": "A taxa Selic atual é 14.25%.",
        },
        status=200,
    )

    client = _make_test_client()
    resp = client.get("/v1/bcb/selic", format="context")

    assert resp.context == "A taxa Selic atual é 14.25%."
    assert "format=context" in responses.calls[0].request.url


@responses.activate
def test_query_params_fields_and_temporal():
    responses.get(
        "https://databr.api.br/v1/economia/ipca",
        json={
            "source": "ibge_sidra",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": False,
            "cache_age_seconds": 0,
            "cost_usdc": "0.003",
            "data": {"valor": "0.52"},
        },
        status=200,
    )

    client = _make_test_client()
    client.get(
        "/v1/economia/ipca", fields=["valor"], since="2026-01-01", until="2026-02-01"
    )

    url = responses.calls[0].request.url
    assert "fields=valor" in url
    assert "since=2026-01-01" in url
    assert "until=2026-02-01" in url


@responses.activate
def test_post_request():
    """POST requests forward body and params correctly."""
    responses.post(
        "https://databr.api.br/v1/carteira/risco",
        json={
            "source": "databr_risk",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": False,
            "cache_age_seconds": 0,
            "cost_usdc": "0.010",
            "data": {"risco": "medio"},
        },
        status=200,
    )

    client = _make_test_client()
    resp = client.post("/v1/carteira/risco", body={"cnpjs": ["12345678000190"]})

    assert resp.data == {"risco": "medio"}
    assert resp.cost_usdc == "0.010"


@responses.activate
def test_502_raises_payment_error():
    """502 from the API indicates a payment settlement failure."""
    from databr.exceptions import PaymentError

    responses.get(
        "https://databr.api.br/v1/bcb/selic",
        json={"error": "payment settlement failed"},
        status=502,
    )

    client = _make_test_client()
    with pytest.raises(PaymentError) as exc_info:
        client.get("/v1/bcb/selic")
    assert exc_info.value.status_code == 502


@responses.activate
def test_context_manager():
    """Client works as a context manager and closes cleanly."""
    responses.get(
        "https://databr.api.br/v1/bcb/selic",
        json={
            "source": "bcb_sgs",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": False,
            "cache_age_seconds": 0,
            "cost_usdc": "0.003",
            "data": {"valor": "14.25"},
        },
        status=200,
    )

    with _make_test_client() as client:
        resp = client.get("/v1/bcb/selic")
        assert resp.source == "bcb_sgs"


@responses.activate
def test_fields_as_string():
    """fields param can be a plain comma-separated string."""
    responses.get(
        "https://databr.api.br/v1/economia/ipca",
        json={
            "source": "ibge_sidra",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": False,
            "cache_age_seconds": 0,
            "cost_usdc": "0.003",
            "data": {"valor": "0.52"},
        },
        status=200,
    )

    client = _make_test_client()
    client.get("/v1/economia/ipca", fields="valor,data")

    url = responses.calls[0].request.url
    assert "fields=valor%2Cdata" in url or "fields=valor,data" in url


@responses.activate
def test_limit_and_offset_params():
    """Pagination params are forwarded as strings."""
    responses.get(
        "https://databr.api.br/v1/economia/ipca",
        json={
            "source": "ibge_sidra",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": False,
            "cache_age_seconds": 0,
            "cost_usdc": "0.003",
            "data": [{"valor": "0.52"}],
        },
        status=200,
    )

    client = _make_test_client()
    client.get("/v1/economia/ipca", limit=10, offset=20)

    url = responses.calls[0].request.url
    assert "limit=10" in url
    assert "offset=20" in url


@responses.activate
def test_custom_base_url():
    """Client respects custom base_url."""
    responses.get(
        "http://localhost:8080/v1/bcb/selic",
        json={
            "source": "bcb_sgs",
            "updated_at": "2026-02-23T10:00:00Z",
            "cached": False,
            "cache_age_seconds": 0,
            "cost_usdc": "0.003",
            "data": {"valor": "14.25"},
        },
        status=200,
    )

    client = _make_test_client(base_url="http://localhost:8080")
    resp = client.get("/v1/bcb/selic")
    assert resp.source == "bcb_sgs"
