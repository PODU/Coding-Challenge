"""Minimal JSON-over-HTTP client using only the standard library.

All network access goes through :func:`request_json`, which also decides
which failures are worth retrying.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from typing import Any

from .retry import RetryableError

RETRYABLE_STATUS = {429, 500, 502, 503, 504}


class HttpError(RuntimeError):
    def __init__(self, status: int, body: str, url: str):
        super().__init__(f"HTTP {status} for {url}: {body[:200]}")
        self.status = status
        self.body = body
        self.url = url


def request_json(
    url: str,
    *,
    method: str = "GET",
    headers: dict[str, str] | None = None,
    payload: Any | None = None,
    timeout: float = 60.0,
) -> Any:
    data = None
    hdrs = dict(headers or {})
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        hdrs.setdefault("Content-Type", "application/json")
    req = urllib.request.Request(url, data=data, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        if exc.code in RETRYABLE_STATUS:
            retry_after = None
            header = exc.headers.get("Retry-After") if exc.headers else None
            if header and header.isdigit():
                retry_after = float(header)
            raise RetryableError(
                f"HTTP {exc.code} from {url}", retry_after=retry_after
            ) from exc
        raise HttpError(exc.code, body, url) from exc
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        raise RetryableError(f"network error for {url}: {exc}") from exc
    return json.loads(body) if body.strip() else None
