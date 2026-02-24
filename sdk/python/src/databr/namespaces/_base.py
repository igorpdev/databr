"""Base class for all DataBR namespaces."""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from databr.client import DataBR
    from databr.response import DataBRResponse


class Namespace:
    """Base namespace — provides access to the parent client."""

    def __init__(self, client: DataBR):
        self._client = client

    def _get(self, path: str, **kwargs) -> DataBRResponse:
        return self._client.get(path, **kwargs)

    def _post(self, path: str, body: dict | None = None, **kwargs) -> DataBRResponse:
        return self._client.post(path, body=body, **kwargs)
