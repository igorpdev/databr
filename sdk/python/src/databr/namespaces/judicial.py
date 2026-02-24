"""Judicial namespace."""

from __future__ import annotations

from databr.namespaces._base import Namespace


class JudicialNamespace(Namespace):
    """Judicial — processos, STF, STJ, litigio."""

    def processos(self, doc: str, **kwargs):
        return self._get(f"/v1/judicial/processos/{doc}", **kwargs)

    def stf(self, **kwargs):
        return self._get("/v1/judicial/stf", **kwargs)

    def stj(self, **kwargs):
        return self._get("/v1/judicial/stj", **kwargs)

    def litigio(self, cnpj: str, **kwargs):
        return self._get(f"/v1/litigio/{cnpj}/risco", **kwargs)
