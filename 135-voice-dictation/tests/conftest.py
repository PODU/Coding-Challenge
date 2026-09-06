import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from ccvoice.app import DictationApp  # noqa: E402
from ccvoice.audio import FakeRecorder  # noqa: E402
from ccvoice.config import Config  # noqa: E402
from ccvoice.history import HistoryStore  # noqa: E402
from ccvoice.indicator import SpyIndicator  # noqa: E402
from ccvoice.inject import ClipboardFallbackInjector, FakeClipboard, FakeInjector  # noqa: E402
from ccvoice.netguard import NetworkGuard  # noqa: E402
from ccvoice.ports import AudioClip  # noqa: E402
from ccvoice.stt import EchoEngine  # noqa: E402


@pytest.fixture
def home(tmp_path, monkeypatch):
    """An isolated ~/.ccvoice for the duration of a test."""
    root = tmp_path / "ccvoice-home"
    root.mkdir()
    monkeypatch.setenv("CCVOICE_HOME", str(root))
    return root


@pytest.fixture
def config():
    return Config(live_preview=False, max_recording_s=0)


@pytest.fixture
def store(home):
    with HistoryStore(home / "history.db") as store:
        yield store


class Harness:
    """The whole app wired to fakes, plus handles on every part of it."""

    def __init__(self, config, *, text="hello world", fail_injection="", history=None, guard=None):
        self.recorder = FakeRecorder()
        self.engine = EchoEngine(text)
        self.indicator = SpyIndicator()
        self.primary = FakeInjector(fail_with=fail_injection)
        self.clipboard = FakeClipboard()
        self.injector = ClipboardFallbackInjector(self.primary, self.clipboard, self.indicator.notify)
        self.guard = guard
        self.app = DictationApp(
            config=config,
            recorder=self.recorder,
            engine=self.engine,
            injector=self.injector,
            indicator=self.indicator,
            history=history,
            guard=guard,
        )

    def speak(self, seconds=2.0):
        """One full press-speak-release cycle."""
        self.app.on_press()
        self.recorder.feed_seconds(seconds)
        return self.app.on_release()


@pytest.fixture
def harness(config):
    def make(**kwargs):
        return Harness(kwargs.pop("config", config), **kwargs)

    return make


@pytest.fixture
def clip():
    return AudioClip.silence(1.5)


@pytest.fixture
def guard():
    return NetworkGuard()
