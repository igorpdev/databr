"""Mercado financeiro namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class MercadoNamespace(Namespace):
    """Mercado — acoes, fundos, cotas, fatos relevantes, indices."""

    def acoes(self, ticker: str, **kwargs):
        return self._get(f"/v1/mercado/acoes/{ticker}", **kwargs)

    def fundos(self, cnpj: str, **kwargs):
        return self._get(f"/v1/mercado/fundos/{cnpj}", **kwargs)

    def cotas(self, cnpj: str, **kwargs):
        return self._get(f"/v1/mercado/fundos/{cnpj}/cotas", **kwargs)

    def fatos(self, **kwargs):
        return self._get("/v1/mercado/fatos-relevantes", **kwargs)

    def indices(self, **kwargs):
        return self._get("/v1/mercado/indices/ibovespa", **kwargs)

    def competicao(self, cnae: str, **kwargs):
        return self._get(f"/v1/mercado/{cnae}/competicao", **kwargs)
