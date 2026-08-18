"""Retrying with exponential backoff. Tests pass a no-op sleep."""

from __future__ import annotations

import time
from typing import Callable, TypeVar

T = TypeVar("T")


class RetryableError(RuntimeError):
    """A failure that may go away if the call is tried again.

    ``retry_after`` is the server's Retry-After value in seconds, if it sent
    one; it replaces the backoff delay.
    """

    def __init__(self, message: str, retry_after: float | None = None):
        super().__init__(message)
        self.retry_after = retry_after


def with_retries(
    fn: Callable[[], T],
    *,
    attempts: int = 3,
    base_delay: float = 1.0,
    max_delay: float = 30.0,
    sleep: Callable[[float], None] = time.sleep,
    on_retry: Callable[[int, Exception], None] | None = None,
) -> T:
    """Call ``fn``, retrying up to ``attempts`` times in all.

    Only :class:`RetryableError` is retried. Anything else, such as a 401,
    won't fix itself and is raised straight away.
    """
    last: Exception | None = None
    for attempt in range(1, attempts + 1):
        try:
            return fn()
        except RetryableError as exc:
            last = exc
            if attempt == attempts:
                break
            delay = min(base_delay * (2 ** (attempt - 1)), max_delay)
            if exc.retry_after is not None:
                delay = exc.retry_after
            if on_retry:
                on_retry(attempt, exc)
            sleep(delay)
    assert last is not None
    raise last
