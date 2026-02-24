"""Saude namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class SaudeNamespace(Namespace):
    """Saude — medicamentos, operadoras, estabelecimentos."""

    def medicamentos(self, registro: str, **kwargs):
        return self._get(f"/v1/saude/medicamentos/{registro}", **kwargs)

    def operadoras(self, **kwargs):
        return self._get("/v1/saude/planos", **kwargs)

    def estabelecimentos(self, cnes: str | None = None, **kwargs):
        if cnes:
            return self._get(f"/v1/saude/estabelecimentos/{cnes}", **kwargs)
        return self._get("/v1/saude/estabelecimentos", **kwargs)
