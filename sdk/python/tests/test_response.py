"""Tests for DataBRResponse parsing."""

from datetime import datetime, timezone

from databr.response import DataBRResponse


def test_parse_success_response():
    raw = {
        "source": "bcb_sgs",
        "updated_at": "2026-02-23T10:00:00Z",
        "cached": True,
        "cache_age_seconds": 3600,
        "cost_usdc": "0.003",
        "data": {"valor": "14.25"},
    }
    resp = DataBRResponse.from_dict(raw)
    assert resp.source == "bcb_sgs"
    assert resp.data == {"valor": "14.25"}
    assert resp.cost_usdc == "0.003"
    assert resp.cached is True
    assert resp.cache_age_seconds == 3600
    assert resp.context is None
    assert resp.updated_at == datetime(2026, 2, 23, 10, 0, 0, tzinfo=timezone.utc)


def test_parse_context_response():
    raw = {
        "source": "bcb_sgs",
        "updated_at": "2026-02-23T10:00:00Z",
        "cached": False,
        "cache_age_seconds": 0,
        "cost_usdc": "0.005",
        "context": "A taxa Selic atual é 14.25% ao ano.",
    }
    resp = DataBRResponse.from_dict(raw)
    assert resp.context == "A taxa Selic atual é 14.25% ao ano."
    assert resp.data is None


def test_raw_preserved():
    raw = {"source": "test", "updated_at": "2026-01-01T00:00:00Z", "cached": False,
           "cache_age_seconds": 0, "cost_usdc": "0.001", "data": {"x": 1}, "extra": "field"}
    resp = DataBRResponse.from_dict(raw)
    assert resp.raw == raw
    assert resp.raw["extra"] == "field"
