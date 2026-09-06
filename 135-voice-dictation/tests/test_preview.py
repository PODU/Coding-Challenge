"""Live transcription preview while the key is held."""

import threading

from ccvoice.audio import FakeRecorder
from ccvoice.config import Config
from ccvoice.ports import Transcript
from ccvoice.stt import EchoEngine, StreamingTranscriber


def make_streamer(engine=None, recorder=None, **kwargs):
    seen: list[str] = []
    recorder = recorder or FakeRecorder()
    streamer = StreamingTranscriber(engine or EchoEngine("one two three four"), recorder, seen.append, **kwargs)
    return streamer, recorder, seen


def test_preview_grows_as_more_audio_arrives():
    streamer, recorder, seen = make_streamer()
    recorder.start()

    recorder.feed_seconds(1.0)
    streamer.poll()
    recorder.feed_seconds(1.0)
    streamer.poll()

    assert seen == ["one two", "one two three four"]
    assert all(len(a) <= len(b) for a, b in zip(seen, seen[1:]))


def test_preview_waits_for_enough_audio():
    streamer, recorder, seen = make_streamer(min_audio_s=0.5)
    recorder.start()
    recorder.feed_seconds(0.2)

    assert streamer.poll() is None
    assert seen == []
    assert streamer.polls == 0


def test_unchanged_partials_are_not_re_emitted():
    streamer, recorder, seen = make_streamer()
    recorder.start()
    recorder.feed_seconds(1.0)

    streamer.poll()
    streamer.poll()

    assert seen == ["one two"]
    assert streamer.polls == 2 and streamer.emitted == 1


def test_a_failing_preview_never_kills_the_recording():
    class Exploding:
        model_id = "boom"

        def transcribe(self, clip, *, partial=False):
            raise RuntimeError("model busy")

    streamer, recorder, seen = make_streamer(engine=Exploding())
    recorder.start()
    recorder.feed_seconds(1.0)

    assert streamer.poll() is None
    assert seen == []
    assert streamer.errors == ["model busy"]


def test_previews_are_single_flight_so_a_slow_model_just_skips():
    """A model slower than the interval must not build a backlog of pending passes."""
    concurrent = []
    lock = threading.Lock()

    class Slow:
        model_id = "slow"

        def transcribe(self, clip, *, partial=False):
            with lock:
                concurrent.append(1)
                assert len(concurrent) == 1, "two preview passes ran at once"
            try:
                return Transcript("tick " * len(concurrent), partial=True)
            finally:
                with lock:
                    concurrent.pop()

    recorder = FakeRecorder()
    recorder.start()
    recorder.feed_seconds(2.0)
    streamer, _r, seen = make_streamer(engine=Slow(), recorder=recorder, interval_s=0.01)

    streamer.start()
    threading.Event().wait(0.2)
    streamer.stop()

    assert streamer.polls >= 2
    assert streamer.errors == []  # the overlap assertion above never tripped
    assert seen  # and it did produce previews


def test_the_app_feeds_previews_to_the_indicator(harness):
    config = Config(live_preview=True, preview_interval_s=0.01, max_recording_s=0)
    h = harness(config=config)

    h.app.on_press()
    h.recorder.feed_seconds(2.0)
    for _ in range(3):
        h.app._streamer.poll()
    outcome = h.app.on_release()

    assert h.indicator.previews == ["hello world"]
    assert outcome.previews == ["hello world"]
    assert outcome.text == "Hello world."


def test_preview_is_skipped_entirely_when_disabled(harness):
    h = harness(config=Config(live_preview=False, max_recording_s=0))
    h.app.on_press()
    assert h.app._streamer is None
    h.recorder.feed_seconds(1.0)
    h.app.on_release()
    assert h.indicator.previews == []
