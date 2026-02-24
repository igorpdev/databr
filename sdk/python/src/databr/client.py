"""DataBR SDK client with automatic x402 payment."""

from __future__ import annotations

import requests as req_lib
from eth_account import Account

from x402 import x402ClientSync
from x402.http.clients import x402_requests
from x402.mechanisms.evm import EthAccountSigner
from x402.mechanisms.evm.exact.register import register_exact_evm_client

from databr.constants import DEFAULT_BASE_URL, DEFAULT_TIMEOUT
from databr.exceptions import APIError, NotFoundError, PaymentError, RateLimitError
from databr.response import DataBRResponse


class DataBR:
    """DataBR API client with automatic x402 payment.

    Usage::

        client = DataBR(private_key="0x...")
        selic = client.bcb.selic()
        empresa = client.get("/v1/empresas/12345678000190")
    """

    def __init__(
        self,
        private_key: str,
        base_url: str = DEFAULT_BASE_URL,
        network: str = "mainnet",
        timeout: int = DEFAULT_TIMEOUT,
    ):
        self._base_url = base_url.rstrip("/")
        self._timeout = timeout

        # Set up x402 client with EVM signer
        self._x402 = x402ClientSync()
        account = Account.from_key(private_key)
        register_exact_evm_client(self._x402, EthAccountSigner(account))

        # Create x402-aware requests session (returns a plain Session, not a context manager)
        self._session = x402_requests(self._x402)

        # Lazy-loaded namespaces (imported here to avoid circular imports)
        self._ns_cache: dict[str, object] = {}

    def close(self) -> None:
        """Close the underlying HTTP session."""
        self._session.close()

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()

    def get(self, path: str, **kwargs) -> DataBRResponse:
        """Make a GET request to the DataBR API.

        Args:
            path: API path (e.g., "/v1/bcb/selic")
            **kwargs: Query parameters -- format, fields, since, until, limit, offset

        Returns:
            DataBRResponse with parsed data.
        """
        url = f"{self._base_url}{path}"
        params = self._build_params(kwargs)
        resp = self._session.get(url, params=params, timeout=self._timeout)
        return self._handle_response(resp)

    def post(self, path: str, body: dict | None = None, **kwargs) -> DataBRResponse:
        """Make a POST request to the DataBR API.

        Args:
            path: API path (e.g., "/v1/carteira/risco")
            body: JSON request body
            **kwargs: Query parameters
        """
        url = f"{self._base_url}{path}"
        params = self._build_params(kwargs)
        resp = self._session.post(url, json=body, params=params, timeout=self._timeout)
        return self._handle_response(resp)

    def _build_params(self, kwargs: dict) -> dict:
        params = {}
        if "format" in kwargs:
            params["format"] = kwargs["format"]
        if "fields" in kwargs:
            fields = kwargs["fields"]
            params["fields"] = ",".join(fields) if isinstance(fields, list) else fields
        if "since" in kwargs:
            params["since"] = kwargs["since"]
        if "until" in kwargs:
            params["until"] = kwargs["until"]
        if "limit" in kwargs:
            params["limit"] = str(kwargs["limit"])
        if "offset" in kwargs:
            params["offset"] = str(kwargs["offset"])
        return params

    def _handle_response(self, resp: req_lib.Response) -> DataBRResponse:
        if resp.status_code == 404:
            msg = resp.json().get("error", "not found") if resp.text else "not found"
            raise NotFoundError(msg, status_code=404)

        if resp.status_code == 429:
            retry_after = resp.headers.get("Retry-After")
            raise RateLimitError(
                "rate limit exceeded",
                retry_after=int(retry_after) if retry_after else None,
            )

        if resp.status_code == 502:
            raise PaymentError(
                resp.json().get("error", "payment failed"), status_code=502
            )

        if resp.status_code >= 400:
            msg = (
                resp.json().get("error", f"HTTP {resp.status_code}")
                if resp.text
                else f"HTTP {resp.status_code}"
            )
            raise APIError(msg, status_code=resp.status_code)

        return DataBRResponse.from_dict(resp.json())

    # --- Namespace accessors (lazy) ---

    @property
    def bcb(self):
        from databr.namespaces.bcb import BCBNamespace

        return self._get_ns("bcb", BCBNamespace)

    @property
    def empresas(self):
        from databr.namespaces.empresas import EmpresasNamespace

        return self._get_ns("empresas", EmpresasNamespace)

    @property
    def economia(self):
        from databr.namespaces.economia import EconomiaNamespace

        return self._get_ns("economia", EconomiaNamespace)

    @property
    def mercado(self):
        from databr.namespaces.mercado import MercadoNamespace

        return self._get_ns("mercado", MercadoNamespace)

    @property
    def compliance(self):
        from databr.namespaces.compliance import ComplianceNamespace

        return self._get_ns("compliance", ComplianceNamespace)

    @property
    def judicial(self):
        from databr.namespaces.judicial import JudicialNamespace

        return self._get_ns("judicial", JudicialNamespace)

    @property
    def legislativo(self):
        from databr.namespaces.legislativo import LegislativoNamespace

        return self._get_ns("legislativo", LegislativoNamespace)

    @property
    def ambiental(self):
        from databr.namespaces.ambiental import AmbientalNamespace

        return self._get_ns("ambiental", AmbientalNamespace)

    @property
    def transparencia(self):
        from databr.namespaces.transparencia import TransparenciaNamespace

        return self._get_ns("transparencia", TransparenciaNamespace)

    @property
    def saude(self):
        from databr.namespaces.saude import SaudeNamespace

        return self._get_ns("saude", SaudeNamespace)

    @property
    def energia(self):
        from databr.namespaces.energia import EnergiaNamespace

        return self._get_ns("energia", EnergiaNamespace)

    @property
    def transporte(self):
        from databr.namespaces.transporte import TransporteNamespace

        return self._get_ns("transporte", TransporteNamespace)

    @property
    def educacao(self):
        from databr.namespaces.educacao import EducacaoNamespace

        return self._get_ns("educacao", EducacaoNamespace)

    @property
    def emprego(self):
        from databr.namespaces.emprego import EmpregoNamespace

        return self._get_ns("emprego", EmpregoNamespace)

    @property
    def comercio(self):
        from databr.namespaces.comercio import ComercioNamespace

        return self._get_ns("comercio", ComercioNamespace)

    def _get_ns(self, name: str, cls: type):
        if name not in self._ns_cache:
            self._ns_cache[name] = cls(self)
        return self._ns_cache[name]
