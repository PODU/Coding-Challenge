"""Hotkey registration, press to record, release to stop."""

import logging

import pytest

from ccvoice.hotkey import UNSUPPORTED_HINTS, VK_CODES, FakeHotkeyListener, normalise
from ccvoice.ports import HotkeyUnavailable


def test_two_hotkey_options_are_offered_by_default():
    from ccvoice.config import Config

    keys = Config().hotkeys()
    assert len(keys) >= 2
    assert Config().hotkey == "right ctrl"


def test_register_binds_both_edges_and_start_arms_the_listener(config, harness):
    h = harness()
    listener = FakeHotkeyListener()
    h.app.hotkey = listener
    h.app.start()

    assert listener.started
    assert listener.combo == normalise(",".join(config.hotkeys()))


def test_press_starts_recording_and_release_stops_it(harness):
    h = harness()
    h.app.hotkey = FakeHotkeyListener()
    h.app.start()

    h.app.hotkey.press()
    assert h.recorder.is_recording
    h.recorder.feed_seconds(1.0)

    h.app.hotkey.release()
    assert not h.recorder.is_recording
    assert h.recorder.starts == 1 and h.recorder.stops == 1
    assert h.app.outcomes[-1].text == "Hello world."


def test_a_second_press_while_recording_is_ignored(harness):
    h = harness()
    h.app.on_press()
    h.app.on_press()
    h.recorder.feed_seconds(0.5)
    h.app.on_release()
    assert h.recorder.starts == 1


def test_release_without_press_does_nothing(harness):
    h = harness()
    assert h.app.on_release() is None


def test_duration_and_events_are_logged(harness, caplog):
    h = harness()
    with caplog.at_level(logging.INFO, logger="ccvoice"):
        h.speak(seconds=2.0)
    messages = [r.getMessage() for r in caplog.records]
    assert any("recording started" in m for m in messages)
    assert any("2.00s of audio" in m for m in messages)


def test_very_short_press_is_discarded(harness):
    h = harness()
    outcome = h.speak(seconds=0.05)
    assert outcome.skipped == "clip too short"
    assert h.primary.injected == []


@pytest.mark.parametrize("name", ["right ctrl", "Right-Ctrl", "RIGHT_CTRL"])
def test_key_names_are_normalised(name):
    assert normalise(name) == "right ctrl"
    assert normalise(name) in VK_CODES


def test_fn_is_rejected_with_an_explanation():
    from ccvoice.hotkey import PynputHotkeyListener, WindowsHotkeyListener

    listener_cls = WindowsHotkeyListener
    try:
        listener = listener_cls()
    except HotkeyUnavailable:
        pytest.importorskip("pynput")
        listener = PynputHotkeyListener()
    with pytest.raises(HotkeyUnavailable) as excinfo:
        listener.register("fn", lambda: None, lambda: None)
    assert "Fn key" in str(excinfo.value)
    assert "fn" in UNSUPPORTED_HINTS


def test_stdin_listener_toggles_press_and_release():
    import io

    from ccvoice.hotkey import StdinHotkeyListener

    events = []
    listener = StdinHotkeyListener(io.StringIO("\n\nq\n"))
    listener.register("", lambda: events.append("press"), lambda: events.append("release"))
    listener.start()
    listener.wait(timeout=2)
    assert events == ["press", "release"]
