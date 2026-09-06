"""The dictation loop.

Hold the hotkey and speak; on release the clip is transcribed, cleaned up, typed into
the focused app and saved to history. Everything it uses is a port, so tests can run
the whole path without a microphone or a model.
"""

from __future__ import annotations

import logging
import threading
import time
from dataclasses import dataclass, field
from typing import Optional

from .config import Config
from .history import HistoryStore
from .netguard import NetworkGuard
from .pipeline import TextPipeline, word_count
from .ports import AudioClip, DictationRecord, InjectionResult, Permission, PermissionState
from .stt import StreamingTranscriber

log = logging.getLogger("ccvoice")

MIN_CLIP_S = 0.25


@dataclass
class DictationOutcome:
    raw_text: str = ""
    text: str = ""
    duration_s: float = 0.0
    previews: list[str] = field(default_factory=list)
    result: Optional[InjectionResult] = None
    record_id: Optional[int] = None
    skipped: str = ""

    @property
    def ok(self) -> bool:
        return bool(self.result and self.result.ok)


class DictationApp:
    def __init__(
        self,
        config: Config,
        recorder,
        engine,
        injector,
        indicator,
        history: Optional[HistoryStore] = None,
        hotkey=None,
        guard: Optional[NetworkGuard] = None,
        preview_engine=None,
        clock=time.monotonic,
    ):
        self.config = config
        self.recorder = recorder
        self.engine = engine
        self.injector = injector
        self.indicator = indicator
        self.history = history
        self.hotkey = hotkey
        self.guard = guard
        self.preview_engine = preview_engine or engine
        self._clock = clock

        self.pipeline = TextPipeline(config)
        self.outcomes: list[DictationOutcome] = []
        self.blocked_reason = ""
        self._streamer: Optional[StreamingTranscriber] = None
        self._previews: list[str] = []
        self._started_at = 0.0
        self._active = False
        self._lock = threading.Lock()
        self._watchdog: Optional[threading.Timer] = None

    def start(self) -> None:
        if self.hotkey is not None:
            combo = ",".join(self.config.hotkeys())
            self.hotkey.register(combo, self.on_press, self.on_release)
            self.hotkey.start()
            log.info("listening for hotkey: %s", combo)

    def stop(self) -> None:
        if self._active:
            self.on_release()
        if self.hotkey is not None:
            self.hotkey.stop()

    def block(self, reason: str) -> None:
        """Suspend dictation, e.g. because a permission was revoked."""
        self.blocked_reason = reason
        log.warning("dictation suspended: %s", reason)
        self.indicator.notify(f"Dictation paused — {reason}")

    def unblock(self) -> None:
        self.blocked_reason = ""
        self.indicator.notify("Permissions restored — dictation is live again.")

    def on_permission_revoked(self, state: PermissionState) -> None:
        which = "Microphone access" if state.permission is Permission.MICROPHONE else "Input injection"
        self.block(f"{which} was revoked. {state.remedy or state.detail}")

    def on_permission_restored(self, _state: PermissionState) -> None:
        if self.blocked_reason:
            self.unblock()

    def on_press(self) -> None:
        with self._lock:
            if self._active:
                return
            if self.blocked_reason:
                self.indicator.notify(f"Cannot record — {self.blocked_reason}")
                return
            self._active = True
            self._previews = []
            self._started_at = self._clock()

        if self.guard is not None:
            self.guard.arm()  # nothing from here to release may touch the network
        self.recorder.start()
        self.indicator.show(self.engine.model_id)
        log.info("recording started")

        if self.config.live_preview:
            self._streamer = StreamingTranscriber(
                self.preview_engine,
                self.recorder,
                self._on_partial,
                interval_s=self.config.preview_interval_s,
            )
            self._streamer.start()

        if self.config.max_recording_s:
            self._watchdog = threading.Timer(self.config.max_recording_s, self._timeout)
            self._watchdog.daemon = True
            self._watchdog.start()

    def _timeout(self) -> None:  # pragma: no cover - timing dependent
        if self._active:
            log.warning("recording hit the %.0fs limit; finishing early", self.config.max_recording_s)
            self.on_release()

    def _on_partial(self, text: str) -> None:
        self._previews.append(text)
        self.indicator.preview(text)

    def on_release(self) -> Optional[DictationOutcome]:
        with self._lock:
            if not self._active:
                return None
            self._active = False
        if self._watchdog is not None:
            self._watchdog.cancel()
            self._watchdog = None
        try:
            clip = self.recorder.stop()
            if self._streamer is not None:
                self._streamer.stop()
                self._streamer = None
            self.indicator.hide()
            held = self._clock() - self._started_at
            log.info("recording stopped: %.2fs held, %.2fs of audio", held, clip.duration_s)
            outcome = self._finish(clip)
        finally:
            if self.guard is not None:
                self.guard.disarm()
        self.outcomes.append(outcome)
        return outcome

    def dictate(self, clip: AudioClip) -> DictationOutcome:
        """Run one clip through the whole path. Used by `ccvoice transcribe`."""
        return self._finish(clip)

    def _finish(self, clip: AudioClip) -> DictationOutcome:
        outcome = DictationOutcome(duration_s=clip.duration_s, previews=list(self._previews))
        if clip.duration_s < MIN_CLIP_S:
            outcome.skipped = "clip too short"
            log.info("ignoring %.2fs clip", clip.duration_s)
            return outcome

        started = self._clock()
        transcript = self.engine.transcribe(clip)
        outcome.raw_text = transcript.text.strip()
        if not outcome.raw_text:
            outcome.skipped = "no speech recognised"
            self.indicator.notify("Nothing was recognised in that clip.")
            return outcome

        outcome.text = self.pipeline.run(outcome.raw_text)
        log.info(
            "transcribed %.2fs in %.2fs with %s: %d words",
            clip.duration_s, self._clock() - started, self.engine.model_id, word_count(outcome.text),
        )

        outcome.result = self.injector.inject(outcome.text)
        if not outcome.result.ok:
            log.error("injection failed: %s", outcome.result.error)

        if self.history is not None:
            outcome.record_id = self.history.record(
                DictationRecord(
                    duration_s=clip.duration_s,
                    model_id=self.engine.model_id,
                    target_app=outcome.result.target_app,
                    raw_text=outcome.raw_text,
                    clean_text=outcome.text,
                    word_count=word_count(outcome.text),
                    injection_method=outcome.result.method,
                )
            )
        return outcome

    def switch_model(self, engine, preview_engine=None) -> None:
        """Swap in another (already downloaded) model without restarting."""
        if preview_engine is not None:
            self.preview_engine = preview_engine
        elif self.preview_engine is self.engine:
            self.preview_engine = engine  # a dedicated preview model is left in place
        self.engine = engine
        self.config.model_id = engine.model_id
        log.info("model switched to %s", engine.model_id)
        self.indicator.notify(f"Model: {engine.model_id}")
