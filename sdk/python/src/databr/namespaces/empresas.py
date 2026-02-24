"""Empresas (CNPJ) namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class EmpresasNamespace(Namespace):
    """Empresas — consulta CNPJ, socios, compliance, due diligence."""

    def consultar(self, cnpj: str, **kwargs):
        return self._get(f"/v1/empresas/{cnpj}", **kwargs)

    def socios(self, cnpj: str, **kwargs):
        return self._get(f"/v1/empresas/{cnpj}/socios", **kwargs)

    def simples(self, cnpj: str, **kwargs):
        return self._get(f"/v1/empresas/{cnpj}/simples", **kwargs)

    def compliance(self, cnpj: str, **kwargs):
        return self._get(f"/v1/empresas/{cnpj}/compliance", **kwargs)

    def perfil_completo(self, cnpj: str, **kwargs):
        return self._get(f"/v1/empresas/{cnpj}/perfil-completo", **kwargs)

    def setor(self, cnpj: str, **kwargs):
        return self._get(f"/v1/empresas/{cnpj}/setor", **kwargs)

    def due_diligence(self, cnpj: str, **kwargs):
        return self._get(f"/v1/empresas/{cnpj}/duediligence", **kwargs)
