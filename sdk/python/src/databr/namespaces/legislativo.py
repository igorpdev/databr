"""Legislativo namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class LegislativoNamespace(Namespace):
    """Legislativo — deputados, senadores, proposicoes, votacoes."""

    def deputados(self, **kwargs):
        return self._get("/v1/legislativo/deputados", **kwargs)

    def deputado(self, id: str, **kwargs):
        return self._get(f"/v1/legislativo/deputados/{id}", **kwargs)

    def senadores(self, **kwargs):
        return self._get("/v1/legislativo/senado/senadores", **kwargs)

    def proposicoes(self, **kwargs):
        return self._get("/v1/legislativo/proposicoes", **kwargs)

    def votacoes(self, **kwargs):
        return self._get("/v1/legislativo/votacoes", **kwargs)
