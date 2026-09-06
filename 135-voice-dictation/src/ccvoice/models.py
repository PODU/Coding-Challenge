"""The model catalogue and the downloader.

This is the only module allowed to open a socket, and only when the user asks for a
model that isn't on disk yet. The fetcher is injectable (`ModelManager(fetcher=...)`)
so tests can cover progress, resume, checksum failures and atomic replacement offline.
"""

from __future__ import annotations

import hashlib
import os
import shutil
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Iterable, Iterator, Optional

from .config import default_home

CHUNK = 1 << 16


@dataclass(frozen=True)
class ModelSpec:
    id: str
    engine: str  # whisper.cpp | faster-whisper | vosk
    filename: str
    url: str
    size_bytes: int
    speed: str  # relative wall-clock for a 10s clip
    accuracy: str
    notes: str = ""
    sha256: Optional[str] = None
    archive: bool = False  # zip that must be expanded into a directory

    @property
    def size_mb(self) -> float:
        return self.size_bytes / 1_000_000


_WHISPER = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"

CATALOG: dict[str, ModelSpec] = {
    spec.id: spec
    for spec in [
        ModelSpec(
            id="whisper-tiny.en",
            engine="whisper.cpp",
            filename="ggml-tiny.en.bin",
            url=_WHISPER + "ggml-tiny.en.bin",
            size_bytes=77_700_000,
            speed="fastest (~0.3x realtime on CPU)",
            accuracy="rough — fine for short commands",
            notes="English only",
        ),
        ModelSpec(
            id="whisper-base.en",
            engine="whisper.cpp",
            filename="ggml-base.en.bin",
            url=_WHISPER + "ggml-base.en.bin",
            size_bytes=147_950_000,
            speed="fast (~0.6x realtime on CPU)",
            accuracy="good — the default balance",
            notes="English only",
        ),
        ModelSpec(
            id="whisper-small.en",
            engine="whisper.cpp",
            filename="ggml-small.en.bin",
            url=_WHISPER + "ggml-small.en.bin",
            size_bytes=487_600_000,
            speed="slower (~1.8x realtime on CPU)",
            accuracy="best of the English set",
            notes="English only; noticeably better on names and jargon",
        ),
        ModelSpec(
            id="vosk-small-en-us",
            engine="vosk",
            filename="vosk-model-small-en-us-0.15",
            url="https://alphacephei.com/vosk/models/vosk-model-small-en-us-0.15.zip",
            size_bytes=40_000_000,
            speed="realtime streaming",
            accuracy="lower than whisper, but truly incremental",
            notes="genuine partial results; best choice for live preview",
            archive=True,
        ),
    ]
}

DEFAULT_MODEL = "whisper-base.en"


@dataclass(frozen=True)
class DownloadProgress:
    model_id: str
    downloaded: int
    total: int
    done: bool = False

    @property
    def fraction(self) -> float:
        return min(1.0, self.downloaded / self.total) if self.total else 0.0

    def bar(self, width: int = 28) -> str:
        filled = int(self.fraction * width)
        pct = f"{self.fraction * 100:5.1f}%"
        mb = f"{self.downloaded / 1_000_000:6.1f}/{self.total / 1_000_000:.1f} MB"
        return f"[{'#' * filled}{'.' * (width - filled)}] {pct}  {mb}"


@dataclass
class FetchResponse:
    chunks: Iterable[bytes]
    total: int
    """Size of the complete file, not of this transfer."""
    resumed: bool = False
    """True when the server honoured our Range request and `chunks` is the remainder."""


Fetcher = Callable[[str, int], FetchResponse]


class ModelError(RuntimeError):
    pass


def http_fetcher(url: str, offset: int = 0) -> FetchResponse:  # pragma: no cover - network
    import urllib.request

    request = urllib.request.Request(url, headers={"User-Agent": "ccvoice/1.0"})
    if offset:
        request.add_header("Range", f"bytes={offset}-")
    response = urllib.request.urlopen(request, timeout=60)
    length = int(response.headers.get("Content-Length") or 0)
    resumed = response.status == 206
    total = length + offset if resumed else length

    def chunks() -> Iterator[bytes]:
        with response:
            while True:
                block = response.read(CHUNK)
                if not block:
                    return
                yield block

    return FetchResponse(chunks(), total, resumed)


class ModelManager:
    """Owns `~/.ccvoice/models/`: what is installed, and getting what is not."""

    def __init__(self, home: Optional[Path] = None, fetcher: Optional[Fetcher] = None):
        self.root = (home or default_home()) / "models"
        self.fetcher: Fetcher = fetcher or http_fetcher

    @staticmethod
    def spec(model_id: str) -> ModelSpec:
        try:
            return CATALOG[model_id]
        except KeyError:
            raise ModelError(f"unknown model {model_id!r}; try: {', '.join(CATALOG)}") from None

    def path(self, model_id: str) -> Path:
        return self.root / self.spec(model_id).filename

    def is_installed(self, model_id: str) -> bool:
        path = self.path(model_id)
        return path.is_dir() if self.spec(model_id).archive else path.is_file()

    def installed(self) -> list[str]:
        return [mid for mid in CATALOG if self.is_installed(mid)]

    def ensure(
        self,
        model_id: str,
        progress: Optional[Callable[[DownloadProgress], None]] = None,
    ) -> Path:
        """Return the local path for `model_id`, downloading it first if need be."""
        spec = self.spec(model_id)
        final = self.path(model_id)
        if self.is_installed(model_id):
            return final

        self.root.mkdir(parents=True, exist_ok=True)
        part = self.root / (spec.filename + ".part")
        offset = part.stat().st_size if part.exists() else 0

        response = self.fetcher(spec.url, offset)
        if offset and not response.resumed:
            part.unlink()  # server ignored Range; start over rather than corrupt the file
            offset = 0
        total = response.total or spec.size_bytes

        downloaded = offset
        with open(part, "ab" if offset else "wb") as fh:
            for block in response.chunks:
                fh.write(block)
                downloaded += len(block)
                if progress:
                    progress(DownloadProgress(model_id, downloaded, total))

        if spec.sha256:
            digest = _sha256(part)
            if digest != spec.sha256:
                part.unlink()
                raise ModelError(f"checksum mismatch for {model_id}: got {digest}")

        if spec.archive:
            self._expand(part, final)
            part.unlink()
        else:
            os.replace(part, final)  # atomic: a half-written model is never visible

        if progress:
            progress(DownloadProgress(model_id, downloaded, total, done=True))
        return final

    @staticmethod
    def _expand(archive: Path, target: Path) -> None:
        staging = target.parent / (target.name + ".unpack")  # not with_suffix: names carry dots
        if staging.exists():
            shutil.rmtree(staging)
        with zipfile.ZipFile(archive) as zf:
            zf.extractall(staging)
        entries = list(staging.iterdir())
        source = entries[0] if len(entries) == 1 and entries[0].is_dir() else staging
        if target.exists():
            shutil.rmtree(target)
        os.replace(source, target)
        if staging.exists():
            shutil.rmtree(staging, ignore_errors=True)

    def remove(self, model_id: str) -> bool:
        path = self.path(model_id)
        if path.is_dir():
            shutil.rmtree(path)
            return True
        if path.is_file():
            path.unlink()
            return True
        return False

    def describe(self) -> list[tuple[str, ModelSpec, bool]]:
        return [(mid, spec, self.is_installed(mid)) for mid, spec in CATALOG.items()]


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for block in iter(lambda: fh.read(CHUNK), b""):
            digest.update(block)
    return digest.hexdigest()
