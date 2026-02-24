"""Emprego namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class EmpregoNamespace(Namespace):
    """Emprego — RAIS, CAGED, mercado de trabalho."""

    def rais(self, **kwargs):
        return self._get("/v1/emprego/rais", **kwargs)

    def caged(self, **kwargs):
        return self._get("/v1/emprego/caged", **kwargs)

    def mercado_trabalho(self, uf: str, **kwargs):
        return self._get(f"/v1/mercado-trabalho/{uf}/analise", **kwargs)
