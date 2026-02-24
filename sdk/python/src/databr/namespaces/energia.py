"""Energia namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class EnergiaNamespace(Namespace):
    """Energia — tarifas, geracao, carga, combustiveis."""

    def tarifas(self, **kwargs):
        return self._get("/v1/energia/tarifas", **kwargs)

    def geracao(self, **kwargs):
        return self._get("/v1/energia/geracao", **kwargs)

    def carga(self, **kwargs):
        return self._get("/v1/energia/carga", **kwargs)

    def combustiveis(self, **kwargs):
        return self._get("/v1/energia/combustiveis", **kwargs)
