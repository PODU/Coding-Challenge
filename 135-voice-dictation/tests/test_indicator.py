"""The recording indicator appears and disappears with the hotkey."""

import io

from ccvoice.indicator import ConsoleIndicator, NullIndicator


def test_indicator_shows_on_press_and_hides_on_release(harness):
    h = harness()
    h.app.on_press()
    assert h.indicator.visible
    assert h.indicator.calls[0] == ("show", "fake-model")

    h.recorder.feed_seconds(1.0)
    h.app.on_release()
    assert not h.indicator.visible
    assert ("hide", "") in h.indicator.calls


def test_indicator_hides_even_when_nothing_was_recognised(harness):
    h = harness(text="")
    outcome = h.speak(seconds=1.0)
    assert outcome.skipped == "no speech recognised"
    assert not h.indicator.visible
    assert h.indicator.notifications == ["Nothing was recognised in that clip."]


def test_console_indicator_draws_a_status_line_and_erases_it():
    ticks = iter([0.0, 0.0, 3.0, 3.0])
    out = io.StringIO()
    indicator = ConsoleIndicator(out, width=60, clock=lambda: next(ticks))

    indicator.show("whisper-base.en")
    indicator.preview("hello there")
    drawn = out.getvalue()
    assert "REC 0:00" in drawn
    assert "whisper-base.en" in drawn
    assert "hello there" in drawn

    out.truncate(0), out.seek(0)
    indicator.hide()
    assert out.getvalue().strip() == ""


def test_console_indicator_truncates_a_long_preview():
    out = io.StringIO()
    indicator = ConsoleIndicator(out, width=50, clock=lambda: 0.0)
    indicator.show("m")
    indicator.preview("word " * 100)
    line = out.getvalue().splitlines()[-1]
    assert len(line) <= 51  # the leading \r plus the fixed width
    assert "..." in line


def test_console_notify_clears_the_line_first():
    out = io.StringIO()
    indicator = ConsoleIndicator(out, width=20, clock=lambda: 0.0)
    indicator.show("m")
    indicator.notify("copied to clipboard")
    assert out.getvalue().endswith("copied to clipboard\n")


def test_null_indicator_is_inert():
    indicator = NullIndicator()
    indicator.show("m"), indicator.preview("x"), indicator.notify("y"), indicator.hide()
