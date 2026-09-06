"""Local speech-to-text engines and the live preview.

Every engine here runs on the machine: whisper.cpp as a subprocess, faster-whisper or
vosk in-process when installed. None of them is allowed to touch the network, and
`netguard` is what proves it.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
import threading
import time
from pathlib import Path
from typing import Callable, Optional

from .config import Config
from .models import CATALOG, ModelManager
from .ports import AudioClip, Transcript, TranscriptionError


class EchoEngine:
    """Test double: returns a fixed transcript, and a plausible prefix for partials."""

    name = "echo"

    def __init__(self, text: str = "hello world", model_id: str = "fake-model", latency_s: float = 0.0):
        self.text = text
        self._model_id = model_id
        self.latency_s = latency_s
        self.calls: list[tuple[float, bool]] = []

    @property
    def model_id(self) -> str:
        return self._model_id

    def set_model(self, model_id: str) -> None:
        self._model_id = model_id

    def transcribe(self, clip: AudioClip, *, partial: bool = False) -> Transcript:
        self.calls.append((clip.duration_s, partial))
        words = self.text.split()
        if partial and words:
            # Roughly two words a second, so a longer snapshot yields more text.
            take = max(1, min(len(words), int(clip.duration_s * 2)))
            return Transcript(" ".join(words[:take]), partial=True, model_id=self._model_id, engine=self.name)
        return Transcript(self.text, model_id=self._model_id, engine=self.name)


class ScriptedEngine:
    """Test double: hands back queued transcripts in order."""

    name = "scripted"

    def __init__(self, finals: list[str], partials: Optional[list[str]] = None, model_id: str = "fake-model"):
        self.finals = list(finals)
        self.partials = list(partials or [])
        self._model_id = model_id

    @property
    def model_id(self) -> str:
        return self._model_id

    def transcribe(self, clip: AudioClip, *, partial: bool = False) -> Transcript:
        queue = self.partials if partial else self.finals
        text = queue.pop(0) if queue else ""
        return Transcript(text, partial=partial, model_id=self._model_id, engine=self.name)


class WhisperCppEngine:
    """Drives a whisper.cpp binary (`whisper-cli`, older builds call it `main`)."""

    name = "whisper.cpp"
    BINARIES = ("whisper-cli", "whisper", "main", "whisper.cpp")

    def __init__(self, model_path: Path, model_id: str, binary: str = "", threads: int = 0):
        self.model_path = Path(model_path)
        self._model_id = model_id
        self.binary = binary or self.find_binary()
        self.threads = threads or max(1, (os.cpu_count() or 4) // 2)
        if not self.binary:
            raise TranscriptionError(
                "whisper.cpp binary not found on PATH (looked for: "
                f"{', '.join(self.BINARIES)}); set `whisper_binary` in config.json"
            )
        if not self.model_path.exists():
            raise TranscriptionError(f"model weights missing: {self.model_path}")

    @classmethod
    def find_binary(cls) -> str:
        for candidate in cls.BINARIES:
            found = shutil.which(candidate)
            if found:
                return found
        return ""

    @property
    def model_id(self) -> str:
        return self._model_id

    def transcribe(self, clip: AudioClip, *, partial: bool = False) -> Transcript:  # pragma: no cover - needs the binary
        started = time.monotonic()
        with tempfile.TemporaryDirectory(prefix="ccvoice-") as tmp:
            wav = Path(tmp) / "clip.wav"
            clip.write_wav(wav)
            argv = [
                self.binary, "-m", str(self.model_path), "-f", str(wav),
                "-t", str(self.threads), "-nt", "-np", "-l", "en",
            ]
            if partial:
                argv += ["-bo", "1", "-bs", "1"]  # greedy: latency beats accuracy for a preview
            try:
                proc = subprocess.run(argv, capture_output=True, text=True, timeout=300)
            except (OSError, subprocess.TimeoutExpired) as exc:
                raise TranscriptionError(f"whisper.cpp failed: {exc}") from exc
        if proc.returncode != 0:
            raise TranscriptionError(f"whisper.cpp exited {proc.returncode}: {proc.stderr.strip()[:400]}")
        return Transcript(
            text=clean_engine_output(proc.stdout),
            partial=partial,
            model_id=self._model_id,
            engine=self.name,
            latency_s=time.monotonic() - started,
        )


class FasterWhisperEngine:
    """In-process CTranslate2 whisper. Faster than the CLI, needs `faster-whisper`."""

    name = "faster-whisper"

    def __init__(self, model_path: Path, model_id: str, compute_type: str = "int8"):
        try:
            from faster_whisper import WhisperModel
        except Exception as exc:  # pragma: no cover - optional dependency
            raise TranscriptionError("faster-whisper is not installed") from exc
        self._model_id = model_id
        self._model = WhisperModel(str(model_path), device="cpu", compute_type=compute_type,
                                   local_files_only=True)

    @property
    def model_id(self) -> str:
        return self._model_id

    def transcribe(self, clip: AudioClip, *, partial: bool = False) -> Transcript:  # pragma: no cover - optional dependency
        started = time.monotonic()
        with tempfile.TemporaryDirectory(prefix="ccvoice-") as tmp:
            wav = Path(tmp) / "clip.wav"
            clip.write_wav(wav)
            segments, _info = self._model.transcribe(str(wav), beam_size=1 if partial else 5, language="en")
            text = " ".join(seg.text.strip() for seg in segments)
        return Transcript(text.strip(), partial=partial, model_id=self._model_id, engine=self.name,
                          latency_s=time.monotonic() - started)


class VoskEngine:
    """Incremental recogniser, which suits the live preview best."""

    name = "vosk"

    def __init__(self, model_path: Path, model_id: str, sample_rate: int = 16_000):
        try:
            from vosk import KaldiRecognizer, Model
        except Exception as exc:  # pragma: no cover - optional dependency
            raise TranscriptionError("vosk is not installed") from exc
        self._model_id = model_id
        self._sample_rate = sample_rate
        self._model = Model(str(model_path))
        self._recognizer_cls = KaldiRecognizer

    @property
    def model_id(self) -> str:
        return self._model_id

    def transcribe(self, clip: AudioClip, *, partial: bool = False) -> Transcript:  # pragma: no cover - optional dependency
        import json

        started = time.monotonic()
        rec = self._recognizer_cls(self._model, clip.sample_rate or self._sample_rate)
        rec.AcceptWaveform(clip.pcm)
        payload = json.loads(rec.PartialResult() if partial else rec.FinalResult())
        text = payload.get("partial" if partial else "text", "")
        return Transcript(text.strip(), partial=partial, model_id=self._model_id, engine=self.name,
                          latency_s=time.monotonic() - started)


def clean_engine_output(raw: str) -> str:
    """whisper.cpp prints one line per segment, sometimes with a [00:00.000] stamp."""
    lines = []
    for line in raw.splitlines():
        line = line.strip()
        if not line or line.startswith("["):
            if "]" in line:
                line = line.split("]", 1)[1].strip()
            if not line:
                continue
        lines.append(line)
    return " ".join(lines).strip()


def build_engine(config: Config, manager: Optional[ModelManager] = None, model_id: str = ""):
    """Instantiate the engine the catalogue names for `model_id`, weights already local."""
    manager = manager or ModelManager()
    model_id = model_id or config.model_id
    spec = ModelManager.spec(model_id)
    if not manager.is_installed(model_id):
        raise TranscriptionError(
            f"model {model_id!r} is not downloaded — run `ccvoice models download {model_id}`"
        )
    path = manager.path(model_id)
    engine = config.engine_hint if config.engine_hint != "auto" else spec.engine
    if engine == "vosk":
        return VoskEngine(path, model_id)
    if engine == "faster-whisper":
        return FasterWhisperEngine(path, model_id)
    return WhisperCppEngine(path, model_id, binary=config.whisper_binary)


class StreamingTranscriber:
    """Re-transcribes a snapshot of the recording at a fixed interval.

    `poll()` runs the engine synchronously, so a model slower than the interval just
    produces fewer previews instead of a backlog. Clock and sleep are injectable for tests.
    """

    def __init__(
        self,
        engine,
        recorder,
        on_partial: Callable[[str], None],
        interval_s: float = 0.7,
        min_audio_s: float = 0.4,
        clock: Callable[[], float] = time.monotonic,
        sleep: Callable[[float], None] = time.sleep,
    ):
        self.engine = engine
        self.recorder = recorder
        self.on_partial = on_partial
        self.interval_s = interval_s
        self.min_audio_s = min_audio_s
        self._clock = clock
        self._sleep = sleep
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None
        self.last_text = ""
        self.polls = 0
        self.emitted = 0
        self.errors: list[str] = []

    def poll(self) -> Optional[str]:
        """One preview pass. Returns the text if it changed, else None."""
        clip = self.recorder.snapshot()
        if clip.duration_s < self.min_audio_s:
            return None
        self.polls += 1
        try:
            transcript = self.engine.transcribe(clip, partial=True)
        except Exception as exc:  # a failed preview must never kill the recording
            self.errors.append(str(exc))
            return None
        text = transcript.text.strip()
        if not text or text == self.last_text:
            return None
        self.last_text = text
        self.emitted += 1
        self.on_partial(text)
        return text

    def start(self) -> None:
        self._stop.clear()
        self.last_text = ""
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()

    def _loop(self) -> None:
        next_at = self._clock() + self.interval_s
        while not self._stop.is_set():
            now = self._clock()
            if now < next_at:
                self._sleep(min(0.05, next_at - now))
                continue
            self.poll()
            next_at = self._clock() + self.interval_s

    def stop(self, timeout: float = 2.0) -> None:
        self._stop.set()
        if self._thread:
            self._thread.join(timeout)
            self._thread = None


def catalogue_rows() -> list[tuple[str, str, str, str]]:
    return [(m.id, m.engine, m.speed, m.accuracy) for m in CATALOG.values()]
