"""Core value types and the six ports the dictation loop is written against.

Nothing in this module imports an adapter, and no adapter imports another adapter.
That is what keeps the whole app testable without a microphone, a model, a focused
window, or a network.
"""

from __future__ import annotations

import io
import wave
from dataclasses import dataclass, field
from enum import Enum
from typing import Callable, Optional, Protocol, runtime_checkable

SAMPLE_RATE = 16_000  # what every whisper build wants
SAMPLE_WIDTH = 2  # 16-bit signed little-endian
CHANNELS = 1


@dataclass(frozen=True)
class AudioClip:
    """A mono PCM16 buffer. The unit of exchange between recorder and engine."""

    pcm: bytes
    sample_rate: int = SAMPLE_RATE
    channels: int = CHANNELS
    sample_width: int = SAMPLE_WIDTH

    @property
    def frames(self) -> int:
        return len(self.pcm) // (self.sample_width * self.channels)

    @property
    def duration_s(self) -> float:
        return self.frames / self.sample_rate if self.sample_rate else 0.0

    def is_empty(self) -> bool:
        return self.frames == 0

    def to_wav_bytes(self) -> bytes:
        buf = io.BytesIO()
        with wave.open(buf, "wb") as wav:
            wav.setnchannels(self.channels)
            wav.setsampwidth(self.sample_width)
            wav.setframerate(self.sample_rate)
            wav.writeframes(self.pcm)
        return buf.getvalue()

    def write_wav(self, path) -> None:
        with open(path, "wb") as fh:
            fh.write(self.to_wav_bytes())

    @classmethod
    def from_wav(cls, path) -> "AudioClip":
        with wave.open(str(path), "rb") as wav:
            return cls(
                pcm=wav.readframes(wav.getnframes()),
                sample_rate=wav.getframerate(),
                channels=wav.getnchannels(),
                sample_width=wav.getsampwidth(),
            )

    @classmethod
    def silence(cls, seconds: float, sample_rate: int = SAMPLE_RATE) -> "AudioClip":
        return cls(pcm=b"\x00" * (int(seconds * sample_rate) * SAMPLE_WIDTH), sample_rate=sample_rate)


@runtime_checkable
class Recorder(Protocol):
    """Microphone capture. `snapshot` is what makes live preview possible."""

    def start(self) -> None: ...

    def stop(self) -> AudioClip: ...

    def snapshot(self) -> AudioClip:
        """Audio captured so far, without stopping the recording."""

    @property
    def is_recording(self) -> bool: ...


class AudioUnavailable(RuntimeError):
    """No usable capture backend on this machine."""


class KeyEvent(Enum):
    PRESS = "press"
    RELEASE = "release"


@runtime_checkable
class HotkeyListener(Protocol):
    """System-wide push-to-talk key. Delivers press and release, not 'typed'."""

    def register(self, combo: str, on_press: Callable[[], None], on_release: Callable[[], None]) -> None: ...

    def start(self) -> None: ...

    def stop(self) -> None: ...


class HotkeyUnavailable(RuntimeError):
    """Global hotkeys cannot be registered here."""


@runtime_checkable
class Indicator(Protocol):
    """The floating 'I am listening' affordance (step 2) and preview surface (step 5)."""

    def show(self, model_id: str) -> None: ...

    def preview(self, text: str) -> None: ...

    def hide(self) -> None: ...

    def notify(self, message: str) -> None: ...


@dataclass(frozen=True)
class Transcript:
    text: str
    partial: bool = False
    model_id: str = ""
    engine: str = ""
    latency_s: float = 0.0
    confidence: Optional[float] = None


@runtime_checkable
class TranscriptionEngine(Protocol):
    """Local speech-to-text. Implementations must not touch the network."""

    name: str

    def transcribe(self, clip: AudioClip, *, partial: bool = False) -> Transcript: ...

    @property
    def model_id(self) -> str: ...


class TranscriptionError(RuntimeError):
    pass


@dataclass(frozen=True)
class InjectionResult:
    ok: bool
    method: str
    error: str = ""
    target_app: str = ""

    @property
    def fell_back(self) -> bool:
        return self.ok and self.method == "clipboard"


@runtime_checkable
class TextInjector(Protocol):
    """Puts text at the caret of whatever window has focus."""

    name: str

    def available(self) -> bool: ...

    def inject(self, text: str) -> InjectionResult: ...

    def focused_app(self) -> str: ...


@runtime_checkable
class Clipboard(Protocol):
    def copy(self, text: str) -> bool: ...


class Permission(Enum):
    MICROPHONE = "microphone"
    INPUT_INJECTION = "input_injection"


@dataclass(frozen=True)
class PermissionState:
    permission: Permission
    granted: bool
    detail: str = ""
    remedy: str = ""
    unknown: bool = False
    """True when the platform gives us no way to query; treated as granted, reported as unknown."""


@runtime_checkable
class PermissionChecker(Protocol):
    def check(self, permission: Permission) -> PermissionState: ...

    def request(self, permission: Permission) -> None:
        """Best-effort nudge: open the OS settings pane, trigger the consent prompt."""


@dataclass
class DictationRecord:
    id: Optional[int] = None
    timestamp: str = ""
    duration_s: float = 0.0
    model_id: str = ""
    target_app: str = ""
    raw_text: str = ""
    clean_text: str = ""
    word_count: int = 0
    injection_method: str = ""
    tags: list[str] = field(default_factory=list)
