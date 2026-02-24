"""Compliance namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class ComplianceNamespace(Namespace):
    """Compliance — verificacao CEIS, CNEP, CEPIM."""

    def verificar(self, cnpj: str, **kwargs):
        return self._get(f"/v1/compliance/{cnpj}", **kwargs)

    def ceis(self, cnpj: str, **kwargs):
        return self._get(f"/v1/compliance/ceis/{cnpj}", **kwargs)

    def cnep(self, cnpj: str, **kwargs):
        return self._get(f"/v1/compliance/cnep/{cnpj}", **kwargs)

    def cepim(self, cnpj: str, **kwargs):
        return self._get(f"/v1/compliance/cepim/{cnpj}", **kwargs)
