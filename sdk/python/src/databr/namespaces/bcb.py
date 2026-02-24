"""Banco Central do Brasil namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class BCBNamespace(Namespace):
    """Banco Central — Selic, cambio, focus, credito, reservas, PIX."""

    def selic(self, **kwargs):
        return self._get("/v1/bcb/selic", **kwargs)

    def cambio(self, moeda: str, **kwargs):
        return self._get(f"/v1/bcb/cambio/{moeda}", **kwargs)

    def focus(self, **kwargs):
        return self._get("/v1/bcb/focus", **kwargs)

    def credito(self, **kwargs):
        return self._get("/v1/bcb/credito", **kwargs)

    def reservas(self, **kwargs):
        return self._get("/v1/bcb/reservas", **kwargs)

    def taxas_credito(self, **kwargs):
        return self._get("/v1/bcb/taxas-credito", **kwargs)

    def pix(self, **kwargs):
        return self._get("/v1/bcb/pix/estatisticas", **kwargs)

    def indicadores(self, serie: str, **kwargs):
        return self._get(f"/v1/bcb/indicadores/{serie}", **kwargs)
