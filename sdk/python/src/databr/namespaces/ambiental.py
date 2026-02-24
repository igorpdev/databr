"""Ambiental namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class AmbientalNamespace(Namespace):
    """Ambiental — desmatamento, prodes, embargos, uso solo, ESG."""

    def desmatamento(self, **kwargs):
        return self._get("/v1/ambiental/desmatamento", **kwargs)

    def prodes(self, **kwargs):
        return self._get("/v1/ambiental/prodes", **kwargs)

    def embargos(self, **kwargs):
        return self._get("/v1/ambiental/embargos", **kwargs)

    def uso_solo(self, **kwargs):
        return self._get("/v1/ambiental/uso-solo", **kwargs)

    def esg(self, cnpj: str, **kwargs):
        return self._get(f"/v1/ambiental/empresa/{cnpj}/esg", **kwargs)

    def risco(self, municipio: str, **kwargs):
        return self._get(f"/v1/ambiental/risco/{municipio}", **kwargs)
