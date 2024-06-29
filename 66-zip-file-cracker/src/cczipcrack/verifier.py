"""Password checking. A :class:`Verifier` opens the archive once and tries to
decrypt one entry per candidate: the right password decrypts and passes the
stored CRC; a wrong one raises. Each worker process makes its own Verifier
because an open ZIP file can't be pickled.
"""

from __future__ import annotations

import zipfile
import zlib
from typing import Any

from .zip_reader import Scheme

# A decrypt with the wrong password raises one of these. They mean "wrong
# password", not a real failure.
_WRONG_PASSWORD_ERRORS: tuple[type[BaseException], ...] = (
    RuntimeError,
    zipfile.BadZipFile,
    zlib.error,
)


class MissingDependencyError(RuntimeError):
    """Raised when an AES archive needs ``pyzipper`` but it is not installed."""


def _open_archive(path: str, scheme: Scheme) -> zipfile.ZipFile:
    if scheme == "aes":
        try:
            import pyzipper  # type: ignore[import-untyped]
        except ImportError as exc:  # pragma: no cover - exercised via CLI gate
            raise MissingDependencyError(
                "AES-encrypted archive requires pyzipper"
            ) from exc
        return pyzipper.AESZipFile(path)
    return zipfile.ZipFile(path)


class Verifier:
    """Checks passwords against one entry of one archive."""

    def __init__(self, path: str, scheme: Scheme, target_name: str) -> None:
        self.path = path
        self.scheme = scheme
        self.target_name = target_name
        self._zf: Any = None

    def _archive(self) -> zipfile.ZipFile:
        if self._zf is None:
            self._zf = _open_archive(self.path, self.scheme)
        return self._zf

    def verify(self, password: str) -> bool:
        """Return ``True`` if *password* decrypts the target entry."""
        try:
            # read() decrypts and checks the entry against its stored CRC.
            self._archive().read(self.target_name, pwd=password.encode("utf-8"))
            return True
        except _WRONG_PASSWORD_ERRORS:
            return False

    def close(self) -> None:
        if self._zf is not None:
            self._zf.close()
            self._zf = None

    def __enter__(self) -> "Verifier":
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

