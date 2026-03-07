"""Recent conversation turns per session, so follow-up questions make sense.

Sessions expire after a TTL (24h by default). The in-memory version takes a
clock so tests can check expiry without sleeping.
"""

from __future__ import annotations

import time
from typing import Callable, Protocol, runtime_checkable

from .models import Turn


@runtime_checkable
class SessionMemory(Protocol):
    def append(self, session_id: str, turn: Turn) -> None: ...

    def recent(self, session_id: str, n: int) -> list[Turn]: ...

    def clear(self, session_id: str | None = None) -> None: ...


class InMemorySessionMemory:
    def __init__(self, *, ttl: int = 86_400, clock: Callable[[], float] = time.time) -> None:
        self._ttl = ttl
        self._clock = clock
        # session_id -> (last_write_time, [turns])
        self._store: dict[str, tuple[float, list[Turn]]] = {}

    def _expired(self, session_id: str) -> bool:
        entry = self._store.get(session_id)
        if entry is None:
            return True
        last, _ = entry
        return (self._clock() - last) >= self._ttl

    def append(self, session_id: str, turn: Turn) -> None:
        now = self._clock()
        turns = [] if self._expired(session_id) else self._store[session_id][1]
        turns.append(turn)
        self._store[session_id] = (now, turns)

    def recent(self, session_id: str, n: int) -> list[Turn]:
        if n <= 0 or self._expired(session_id):
            self._store.pop(session_id, None)
            return []
        _, turns = self._store[session_id]
        return turns[-n:]

    def clear(self, session_id: str | None = None) -> None:
        if session_id is None:
            self._store.clear()
        else:
            self._store.pop(session_id, None)


class RedisSessionMemory:  # pragma: no cover - requires Redis
    """One Redis list per session; the key's TTL is refreshed on each append."""

    def __init__(
        self,
        *,
        url: str = "redis://localhost:6379",
        prefix: str = "rtfm:session:",
        ttl: int = 86_400,
        client=None,
    ) -> None:
        self._prefix = prefix
        self._ttl = ttl
        if client is not None:
            self._r = client
        else:
            try:
                import redis  # type: ignore
            except Exception as exc:
                raise RuntimeError(
                    "RedisSessionMemory requires the optional 'redis' dependency."
                ) from exc
            self._r = redis.Redis.from_url(url)

    def _key(self, session_id: str) -> str:
        return f"{self._prefix}{session_id}"

    def append(self, session_id: str, turn: Turn) -> None:
        import json

        key = self._key(session_id)
        self._r.rpush(key, json.dumps({"question": turn.question, "answer": turn.answer}))
        self._r.expire(key, self._ttl)

    def recent(self, session_id: str, n: int) -> list[Turn]:
        import json

        if n <= 0:
            return []
        items = (json.loads(raw) for raw in self._r.lrange(self._key(session_id), -n, -1))
        return [Turn(question=d["question"], answer=d["answer"]) for d in items]

    def clear(self, session_id: str | None = None) -> None:
        if session_id is None:
            keys = self._r.keys(f"{self._prefix}*")
            if keys:
                self._r.delete(*keys)
        else:
            self._r.delete(self._key(session_id))
