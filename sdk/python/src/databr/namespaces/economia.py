"""Economia namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class EconomiaNamespace(Namespace):
    """Economia — IPCA, PIB, panorama."""

    def ipca(self, **kwargs):
        return self._get("/v1/economia/ipca", **kwargs)

    def pib(self, **kwargs):
        return self._get("/v1/economia/pib", **kwargs)

    def panorama(self, **kwargs):
        return self._get("/v1/economia/panorama", **kwargs)
