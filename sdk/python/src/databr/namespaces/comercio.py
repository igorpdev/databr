"""Comercio exterior namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class ComercioNamespace(Namespace):
    """Comercio — exportacoes, importacoes."""

    def exportacoes(self, **kwargs):
        return self._get("/v1/comercio/exportacoes", **kwargs)

    def importacoes(self, **kwargs):
        return self._get("/v1/comercio/importacoes", **kwargs)
