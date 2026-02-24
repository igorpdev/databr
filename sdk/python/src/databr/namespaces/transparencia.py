"""Transparencia namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class TransparenciaNamespace(Namespace):
    """Transparencia — licitacoes, contratos, servidores, orcamento, TCU, DOU."""

    def licitacoes(self, **kwargs):
        return self._get("/v1/pncp/orgaos", **kwargs)

    def contratos(self, **kwargs):
        return self._get("/v1/transparencia/contratos", **kwargs)

    def servidores(self, **kwargs):
        return self._get("/v1/transparencia/servidores", **kwargs)

    def orcamento(self, **kwargs):
        return self._get("/v1/transparencia/transferencias", **kwargs)

    def tcu(self, **kwargs):
        return self._get("/v1/tcu/acordaos", **kwargs)

    def dou(self, query: str, **kwargs):
        return self._get("/v1/dou/busca", query=query, **kwargs)
