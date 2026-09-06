"""End to end: hotkey down, speech, hotkey up, text at the cursor — all offline."""

from ccvoice.app import DictationApp
from ccvoice.audio import FakeRecorder
from ccvoice.config import Config
from ccvoice.history import HistoryStore
from ccvoice.hotkey import FakeHotkeyListener
from ccvoice.indicator import SpyIndicator
from ccvoice.inject import ClipboardFallbackInjector, FakeClipboard, FakeInjector
from ccvoice.netguard import NetworkGuard
from ccvoice.permissions import FakePermissionChecker, PermissionMonitor
from ccvoice.ports import Permission
from ccvoice.stt import ScriptedEngine

SPEECH = (
    "um so here is the plan first ship the fix second tell the team "
    "new paragraph i will send clod code the the notes period"
)

EXPECTED = (
    "So here is the plan:\n"
    "1. Ship the fix\n"
    "2. Tell the team\n"
    "\n"
    "I will send Claude Code the notes."
)


def build(home, *, fail_injection=""):
    config = Config(
        live_preview=True,
        preview_interval_s=0.01,
        max_recording_s=0,
        dictionary={"clod code": "Claude Code"},
    )
    engine = ScriptedEngine([SPEECH], partials=["um so here is", "um so here is the plan first"],
                            model_id="whisper-base.en")
    recorder = FakeRecorder()
    indicator = SpyIndicator()
    primary = FakeInjector(fail_with=fail_injection, app="Notepad")
    clipboard = FakeClipboard()
    guard = NetworkGuard()
    app = DictationApp(
        config=config,
        recorder=recorder,
        engine=engine,
        injector=ClipboardFallbackInjector(primary, clipboard, indicator.notify),
        indicator=indicator,
        history=HistoryStore(home / "history.db"),
        hotkey=FakeHotkeyListener(),
        guard=guard,
    )
    return app, recorder, indicator, primary, clipboard, guard


def test_a_full_dictation_from_keypress_to_cursor(home):
    app, recorder, indicator, primary, _clipboard, guard = build(home)
    app.start()

    app.hotkey.press()
    recorder.feed_seconds(3.0)
    app._streamer.poll()
    app._streamer.poll()
    app.hotkey.release()

    outcome = app.outcomes[-1]
    assert outcome.text == EXPECTED
    assert primary.injected == [EXPECTED]
    assert outcome.raw_text == SPEECH

    # step 2 + step 5: shown, previewed, hidden — in that order
    kinds = [kind for kind, _ in indicator.calls]
    assert kinds[0] == "show" and kinds[-1] == "hide"
    assert indicator.previews == ["um so here is", "um so here is the plan first"]

    # step 3: nothing tried to leave the machine, and the guard was released
    assert guard.attempts == [] and not guard.armed

    # steps 14 + 15
    store = app.history
    assert [r.clean_text for r in store.search("claude")] == [EXPECTED]
    stats = store.stats()
    assert stats.sessions == 1 and stats.words == 20
    assert stats.by_app == {"Notepad": 20}
    app.stop()


def test_the_same_dictation_falls_back_to_the_clipboard_when_typing_is_refused(home):
    app, recorder, indicator, _primary, clipboard, _guard = build(home, fail_injection="secure field")
    app.start()

    app.hotkey.press()
    recorder.feed_seconds(2.0)
    app.hotkey.release()

    assert clipboard.contents == [EXPECTED]
    assert app.outcomes[-1].result.method == "clipboard"
    assert any("Ctrl+V" in note for note in indicator.notifications)
    assert app.history.stats().clipboard_fallbacks == 1
    app.stop()


def test_two_dictations_in_a_row_accumulate_history(home):
    app, recorder, _indicator, primary, _clipboard, _guard = build(home)
    app.engine = ScriptedEngine(["first one here", "second one here"])
    app.start()

    for _ in range(2):
        app.hotkey.press()
        recorder.feed_seconds(1.0)
        app.hotkey.release()

    assert primary.injected == ["First one here.", "Second one here."]
    assert app.history.count() == 2
    assert app.history.stats().sessions == 2
    app.stop()


def test_a_revoked_permission_stops_dictation_mid_session(home):
    app, recorder, indicator, primary, _clipboard, _guard = build(home)
    checker = FakePermissionChecker()
    monitor = PermissionMonitor(checker, app.on_permission_revoked,
                                on_restored=app.on_permission_restored)
    monitor.poll()
    app.start()

    checker.revoke(Permission.INPUT_INJECTION)
    monitor.poll()

    app.hotkey.press()
    recorder.feed_seconds(1.0)
    app.hotkey.release()

    assert primary.injected == []
    assert app.history.count() == 0
    assert any("paused" in note.lower() for note in indicator.notifications)
    app.stop()


def test_stopping_the_app_finishes_an_in_flight_recording(home):
    app, recorder, _indicator, primary, _clipboard, _guard = build(home)
    app.engine = ScriptedEngine(["caught mid sentence"])
    app.start()

    app.hotkey.press()
    recorder.feed_seconds(1.0)
    app.stop()

    assert primary.injected == ["Caught mid sentence."]
    assert not app.hotkey.started
