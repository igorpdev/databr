"""DataBR — Python SDK for Brazilian public data with x402 payments."""

__version__ = "0.1.0"

from databr.client import DataBR
from databr.exceptions import (
    APIError,
    DataBRError,
    InsufficientBalanceError,
    NotFoundError,
    PaymentError,
    RateLimitError,
)
from databr.response import DataBRResponse

__all__ = [
    "DataBR",
    "DataBRResponse",
    "DataBRError",
    "PaymentError",
    "InsufficientBalanceError",
    "NotFoundError",
    "RateLimitError",
    "APIError",
]
