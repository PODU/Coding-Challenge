"""On-disk configuration: hotkeys, model choice, cleanup rules, custom dictionary."""

from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass, field
from pathlib import Path

DEFAULT_FILLERS = ["um", "uh", "erm", "uhm", "mhm", "hmm", "ah", "er"]
DEFAULT_FILLER_PHRASES = ["you know", "i mean", "sort of", "kind of"]


def default_home() -> Path:
    """App data root. `CCVOICE_HOME` wins, which is how the tests stay off ~."""
    env = os.environ.get("CCVOICE_HOME")
    return Path(env) if env else Path.home() / ".ccvoice"


@dataclass
class Config:
    hotkey: str = "right ctrl"
    alt_hotkey: str = "fn"

    model_id: str = "whisper-base.en"
    engine_hint: str = "auto"  # auto | whisper.cpp | faster-whisper | vosk
    whisper_binary: str = ""  # override for whisper-cli / main

    live_preview: bool = True
    preview_interval_s: float = 0.7
    preview_model_id: str = ""  # blank = same model as the final pass

    cleanup: bool = True
    fillers: list[str] = field(default_factory=lambda: list(DEFAULT_FILLERS))
    filler_phrases: list[str] = field(default_factory=lambda: list(DEFAULT_FILLER_PHRASES))
    collapse_repeats: bool = True
    sentence_case: bool = True
    terminal_period: bool = True
    spoken_commands: bool = True
    detect_lists: bool = True
    list_style: str = "numbered"  # numbered | bulleted
    bullet_marker: str = "-"

    injector: str = "auto"
    clipboard_fallback: bool = True
    inject_chunk: int = 200  # characters per SendInput batch

    setup_complete: bool = False
    permission_poll_s: float = 15.0

    dictionary: dict[str, str] = field(default_factory=dict)

    paranoid: bool = False  # keep the network guard armed for the whole session
    keep_audio: bool = False
    max_recording_s: float = 300.0

    @staticmethod
    def path(home: Path | None = None) -> Path:
        return (home or default_home()) / "config.json"

    @classmethod
    def load(cls, home: Path | None = None) -> "Config":
        path = cls.path(home)
        if not path.exists():
            return cls()
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            return cls()
        # Ignore keys this version doesn't know about.
        known = set(cls.__dataclass_fields__)
        return cls(**{k: v for k, v in data.items() if k in known})

    def save(self, home: Path | None = None) -> Path:
        path = self.path(home)
        path.parent.mkdir(parents=True, exist_ok=True)
        tmp = path.with_suffix(".json.tmp")
        tmp.write_text(json.dumps(asdict(self), indent=2, sort_keys=True), encoding="utf-8")
        os.replace(tmp, path)
        return path

    def add_term(self, spoken: str, written: str) -> None:
        self.dictionary[spoken.strip().lower()] = written.strip()

    def remove_term(self, spoken: str) -> bool:
        return self.dictionary.pop(spoken.strip().lower(), None) is not None

    def hotkeys(self) -> list[str]:
        return [k for k in (self.hotkey, self.alt_hotkey) if k]
