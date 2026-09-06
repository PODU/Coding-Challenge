"""Steps 9 and 10 — inject at the caret, fall back to the clipboard, tell the user."""

from ccvoice.inject import ClipboardFallbackInjector, FakeClipboard, FakeInjector, _expand


def test_the_cleaned_text_is_injected_into_the_focused_app(harness):
    h = harness()
    outcome = h.speak(seconds=1.0)

    assert outcome.result.ok
    assert outcome.result.method == "fake"
    assert outcome.result.target_app == "TestApp"
    assert h.primary.injected == ["Hello world."]
    assert h.clipboard.contents == []


def test_a_refused_injection_falls_back_to_the_clipboard(harness):
    h = harness(fail_injection="UIPI blocked an elevated window")
    outcome = h.speak(seconds=1.0)

    assert outcome.result.ok  # the user still got their text
    assert outcome.result.fell_back
    assert h.clipboard.contents == ["Hello world."]
    assert h.injector.fallbacks == 1


def test_the_fallback_notifies_the_user(harness):
    h = harness(fail_injection="no accessibility permission")
    h.speak(seconds=1.0)

    assert any("clipboard" in note and "Ctrl+V" in note for note in h.indicator.notifications)


def test_when_both_paths_fail_the_result_says_so():
    primary = FakeInjector(fail_with="blocked")
    clipboard = FakeClipboard(works=False)
    notes = []
    injector = ClipboardFallbackInjector(primary, clipboard, notes.append)

    result = injector.inject("text")

    assert not result.ok
    assert result.error == "blocked"
    assert "history" in notes[-1]


def test_an_unavailable_injector_goes_straight_to_the_clipboard():
    injector = ClipboardFallbackInjector(FakeInjector(is_available=False), FakeClipboard())
    result = injector.inject("text")
    assert result.ok and result.method == "clipboard"


def test_with_no_clipboard_configured_the_failure_is_reported():
    notes = []
    injector = ClipboardFallbackInjector(FakeInjector(fail_with="nope"), None, notes.append)
    result = injector.inject("text")
    assert not result.ok
    assert "no clipboard" in notes[-1]


def test_the_fallback_is_recorded_in_history(harness, store):
    h = harness(fail_injection="blocked", history=store)
    h.speak(seconds=1.0)

    assert store.recent()[0].injection_method == "clipboard"
    assert store.stats().clipboard_fallbacks == 1


def test_newlines_are_sent_as_carriage_returns():
    assert _expand("a\nb") == [ord("a"), 0x000D, ord("b")]
    assert _expand("a\r\nb") == [ord("a"), 0x000D, ord("b")]


def test_astral_characters_are_split_into_surrogates():
    units = _expand("\U0001F600")  # an emoji, as dictation apps do get pasted into chat
    assert units == [0xD83D, 0xDE00]


def test_build_injector_wires_the_clipboard_fallback():
    from ccvoice.config import Config
    from ccvoice.inject import build_injector

    injector = build_injector(Config(clipboard_fallback=False))
    assert isinstance(injector, ClipboardFallbackInjector)
    assert injector.clipboard is None
