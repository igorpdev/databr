"""Exception hierarchy for DataBR SDK."""


class DataBRError(Exception):
    """Base exception for all DataBR errors."""

    def __init__(self, message: str, status_code: int | None = None):
        super().__init__(message)
        self.status_code = status_code


class PaymentError(DataBRError):
    """Payment failed during x402 flow."""


class InsufficientBalanceError(PaymentError):
    """Wallet has insufficient USDC balance."""


class NotFoundError(DataBRError):
    """Requested resource not found (404)."""


class RateLimitError(DataBRError):
    """Rate limit exceeded (429)."""

    def __init__(self, message: str, retry_after: int | None = None):
        super().__init__(message, status_code=429)
        self.retry_after = retry_after


class APIError(DataBRError):
    """Unexpected API error (5xx or unknown)."""
