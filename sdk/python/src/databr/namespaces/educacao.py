"""Educacao namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class EducacaoNamespace(Namespace):
    """Educacao — censo escolar."""

    def censo_escolar(self, **kwargs):
        return self._get("/v1/educacao/censo-escolar", **kwargs)
