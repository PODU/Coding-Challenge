"""Microphone capture adapters.

They all have start, snapshot and stop, and all return mono 16 kHz PCM16, which is what
local whisper builds expect. `snapshot()` returns the audio captured so far without
stopping the recording; the live preview depends on it.
"""

from __future__ import annotations

import shutil
import subprocess
import sys
import threading
from typing import Optional

from .ports import CHANNELS, SAMPLE_RATE, SAMPLE_WIDTH, AudioClip, AudioUnavailable


class FakeRecorder:
    """Test double. Audio is fed in by the test instead of by a microphone."""

    def __init__(self, clip: Optional[AudioClip] = None, sample_rate: int = SAMPLE_RATE):
        self._buf = bytearray()
        self._pending = clip.pcm if clip else b""
        self.sample_rate = sample_rate
        self._recording = False
        self.starts = 0
        self.stops = 0

    @property
    def is_recording(self) -> bool:
        return self._recording

    def start(self) -> None:
        self._buf.clear()
        self._recording = True
        self.starts += 1
        if self._pending:
            self._buf.extend(self._pending)

    def feed(self, clip: AudioClip | bytes) -> None:
        """Simulate more speech arriving mid-recording."""
        self._buf.extend(clip.pcm if isinstance(clip, AudioClip) else clip)

    def feed_seconds(self, seconds: float) -> None:
        self.feed(AudioClip.silence(seconds, self.sample_rate))

    def snapshot(self) -> AudioClip:
        return AudioClip(bytes(self._buf), self.sample_rate)

    def stop(self) -> AudioClip:
        self._recording = False
        self.stops += 1
        return AudioClip(bytes(self._buf), self.sample_rate)


class SoundDeviceRecorder:
    """Preferred real backend: PortAudio via the optional `sounddevice` package."""

    name = "sounddevice"

    def __init__(self, sample_rate: int = SAMPLE_RATE, device=None):
        try:
            import sounddevice  # noqa: F401
        except Exception as exc:  # pragma: no cover - depends on the host
            raise AudioUnavailable(f"sounddevice unavailable: {exc}") from exc
        self.sample_rate = sample_rate
        self.device = device
        self._buf = bytearray()
        self._lock = threading.Lock()
        self._stream = None

    @property
    def is_recording(self) -> bool:
        return self._stream is not None

    def start(self) -> None:  # pragma: no cover - needs a microphone
        import sounddevice as sd

        with self._lock:
            self._buf.clear()

        def callback(indata, _frames, _time, status):
            if status:
                pass  # overflows are survivable; the clip just has a gap
            with self._lock:
                self._buf.extend(bytes(indata))

        self._stream = sd.RawInputStream(
            samplerate=self.sample_rate,
            channels=CHANNELS,
            dtype="int16",
            device=self.device,
            callback=callback,
        )
        self._stream.start()

    def snapshot(self) -> AudioClip:  # pragma: no cover - needs a microphone
        with self._lock:
            return AudioClip(bytes(self._buf), self.sample_rate)

    def stop(self) -> AudioClip:  # pragma: no cover - needs a microphone
        stream, self._stream = self._stream, None
        if stream is not None:
            stream.stop()
            stream.close()
        with self._lock:
            return AudioClip(bytes(self._buf), self.sample_rate)


class CommandRecorder:
    """Fallback real backend: pipe raw PCM out of ffmpeg/arecord/sox.

    We read raw s16le rather than WAV because a WAV header's length field is only
    correct once the process exits, and `snapshot()` can't wait for that.
    """

    name = "command"

    def __init__(self, argv: list[str], sample_rate: int = SAMPLE_RATE):
        self.argv = argv
        self.sample_rate = sample_rate
        self._proc: Optional[subprocess.Popen] = None
        self._reader: Optional[threading.Thread] = None
        self._buf = bytearray()
        self._lock = threading.Lock()

    @property
    def is_recording(self) -> bool:
        return self._proc is not None

    def start(self) -> None:  # pragma: no cover - needs a microphone
        with self._lock:
            self._buf.clear()
        self._proc = subprocess.Popen(
            self.argv, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0
        )
        self._reader = threading.Thread(target=self._pump, daemon=True)
        self._reader.start()

    def _pump(self) -> None:  # pragma: no cover - needs a microphone
        assert self._proc and self._proc.stdout
        for chunk in iter(lambda: self._proc.stdout.read(4096), b""):
            with self._lock:
                self._buf.extend(chunk)

    def snapshot(self) -> AudioClip:  # pragma: no cover - needs a microphone
        with self._lock:
            pcm = bytes(self._buf)
        return AudioClip(pcm[: len(pcm) - len(pcm) % SAMPLE_WIDTH], self.sample_rate)

    def stop(self) -> AudioClip:  # pragma: no cover - needs a microphone
        proc, self._proc = self._proc, None
        if proc is not None:
            proc.terminate()
            try:
                proc.wait(timeout=2)
            except subprocess.TimeoutExpired:
                proc.kill()
        if self._reader is not None:
            self._reader.join(timeout=2)
            self._reader = None
        with self._lock:
            pcm = bytes(self._buf)
        return AudioClip(pcm[: len(pcm) - len(pcm) % SAMPLE_WIDTH], self.sample_rate)


def capture_argv(sample_rate: int = SAMPLE_RATE, device: str = "") -> Optional[list[str]]:
    """The best raw-PCM capture command line available on this machine, if any."""
    rate = str(sample_rate)
    if sys.platform == "win32":
        if shutil.which("ffmpeg"):
            src = f"audio={device}" if device else "audio=Microphone"
            return ["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "dshow", "-i", src,
                    "-ar", rate, "-ac", "1", "-f", "s16le", "pipe:1"]
        return None
    if sys.platform == "darwin":
        if shutil.which("ffmpeg"):
            return ["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "avfoundation",
                    "-i", device or ":0", "-ar", rate, "-ac", "1", "-f", "s16le", "pipe:1"]
        return None
    if shutil.which("arecord"):
        argv = ["arecord", "-q", "-f", "S16_LE", "-r", rate, "-c", "1", "-t", "raw"]
        if device:
            argv += ["-D", device]
        return argv
    if shutil.which("ffmpeg"):
        return ["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "alsa",
                "-i", device or "default", "-ar", rate, "-ac", "1", "-f", "s16le", "pipe:1"]
    return None


def pick_recorder(sample_rate: int = SAMPLE_RATE, device: str = ""):
    """Best available capture backend, or `AudioUnavailable` explaining what is missing."""
    try:
        return SoundDeviceRecorder(sample_rate, device or None)
    except AudioUnavailable:
        pass
    argv = capture_argv(sample_rate, device)
    if argv:
        return CommandRecorder(argv, sample_rate)
    raise AudioUnavailable(
        "no capture backend: install `sounddevice` (pip install sounddevice) "
        "or ffmpeg/arecord on PATH"
    )
