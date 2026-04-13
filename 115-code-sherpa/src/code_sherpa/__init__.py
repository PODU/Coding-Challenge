"""Code Sherpa: ask questions about a codebase in plain English."""

from __future__ import annotations

from .models import Answer, Citation, CodeChunk, FileRecord, ScoredChunk
from .config import SherpaConfig

__all__ = [
    "Answer",
    "Citation",
    "CodeChunk",
    "FileRecord",
    "ScoredChunk",
    "SherpaConfig",
    "build_sherpa",
    "__version__",
]

__version__ = "0.1.0"


def build_sherpa(config: "SherpaConfig | None" = None):
    """Return a :class:`Sherpa` wired from ``config`` (or the environment)."""

    from .core.sherpa import Sherpa
    from .config import SherpaConfig as _Cfg

    return Sherpa.from_config(config or _Cfg.from_env())
