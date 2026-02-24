"""Response types for DataBR SDK."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone


@dataclass(frozen=True)
class DataBRResponse:
    """Parsed response from DataBR API."""

    source: str
    data: dict | None
    context: str | None
    updated_at: datetime
    cached: bool
    cache_age_seconds: int
    cost_usdc: str
    raw: dict

    @classmethod
    def from_dict(cls, d: dict) -> DataBRResponse:
        updated_str = d.get("updated_at", "")
        if updated_str.endswith("Z"):
            updated_str = updated_str[:-1] + "+00:00"
        updated_at = datetime.fromisoformat(updated_str) if updated_str else datetime.now(timezone.utc)

        return cls(
            source=d.get("source", ""),
            data=d.get("data"),
            context=d.get("context"),
            updated_at=updated_at,
            cached=d.get("cached", False),
            cache_age_seconds=d.get("cache_age_seconds", 0),
            cost_usdc=d.get("cost_usdc", "0"),
            raw=d,
        )
