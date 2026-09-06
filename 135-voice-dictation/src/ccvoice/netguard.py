"""Make sure dictation never touches the network.

Arming the guard swaps `socket.socket` for a subclass that refuses to connect, so an
accidental request (telemetry, a library fetching a tokenizer) raises instead of sending
anything. The app arms it around each dictation, `--paranoid` keeps it armed for the
whole session, and a test arms it around a full dictate-and-inject cycle.
"""

from __future__ import annotations

import socket
import threading
from contextlib import contextmanager
from typing import Optional

_LOOPBACK = {"127.0.0.1", "::1", "localhost"}


class NetworkBlocked(RuntimeError):
    """Something tried to open a connection while the offline guard was armed."""


class NetworkGuard:
    def __init__(self, allow_loopback: bool = False):
        self.allow_loopback = allow_loopback
        self.attempts: list[str] = []
        self._depth = 0
        self._lock = threading.Lock()
        self._saved: Optional[tuple] = None

    @property
    def armed(self) -> bool:
        return self._depth > 0

    def arm(self) -> None:
        with self._lock:
            self._depth += 1
            if self._depth > 1:
                return
            guard = self

            class GuardedSocket(socket.socket):
                def connect(self, address):
                    guard._check(address)
                    return super().connect(address)

                def connect_ex(self, address):
                    guard._check(address)
                    return super().connect_ex(address)

                def sendto(self, data, *args):
                    if args:
                        guard._check(args[-1])
                    return super().sendto(data, *args)

            def guarded_create_connection(address, *args, **kwargs):
                guard._check(address)
                return _real_create_connection(address, *args, **kwargs)

            _real_create_connection = socket.create_connection
            self._saved = (socket.socket, socket.create_connection)
            socket.socket = GuardedSocket
            socket.create_connection = guarded_create_connection

    def disarm(self) -> None:
        with self._lock:
            if self._depth == 0:
                return
            self._depth -= 1
            if self._depth == 0 and self._saved:
                socket.socket, socket.create_connection = self._saved
                self._saved = None

    def _check(self, address) -> None:
        host = address[0] if isinstance(address, tuple) else str(address)
        self.attempts.append(str(host))
        if self.allow_loopback and str(host) in _LOOPBACK:
            return
        raise NetworkBlocked(
            f"offline guard: refused connection to {host!r}. "
            "Dictation must never leave the machine."
        )

    @contextmanager
    def offline(self):
        self.arm()
        try:
            yield self
        finally:
            self.disarm()


GUARD = NetworkGuard()


@contextmanager
def offline(guard: Optional[NetworkGuard] = None):
    with (guard or GUARD).offline() as g:
        yield g
