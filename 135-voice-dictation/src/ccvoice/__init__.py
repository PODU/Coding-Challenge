"""ccvoice: offline voice dictation (Coding Challenge #135)."""

from .config import Config
from .ports import (
    AudioClip,
    DictationRecord,
    InjectionResult,
    Permission,
    PermissionState,
    Transcript,
)

__version__ = "1.0.0"

__all__ = [
    "AudioClip",
    "Config",
    "DictationRecord",
    "InjectionResult",
    "Permission",
    "PermissionState",
    "Transcript",
    "__version__",
]
