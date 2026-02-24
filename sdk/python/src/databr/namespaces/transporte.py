"""Transporte namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class TransporteNamespace(Namespace):
    """Transporte — aeronaves, transportadores, acidentes."""

    def aeronaves(self, prefixo: str | None = None, **kwargs):
        if prefixo:
            return self._get(f"/v1/transporte/aeronaves/{prefixo}", **kwargs)
        return self._get("/v1/transporte/aeronaves", **kwargs)

    def transportadores(self, rntrc: str | None = None, **kwargs):
        if rntrc:
            return self._get(f"/v1/transporte/transportadores/{rntrc}", **kwargs)
        return self._get("/v1/transporte/transportadores", **kwargs)

    def acidentes(self, **kwargs):
        return self._get("/v1/transporte/acidentes", **kwargs)
