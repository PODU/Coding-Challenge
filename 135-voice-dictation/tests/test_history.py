"""Steps 14 and 15 — searchable history and persistent statistics."""

from ccvoice.history import HistoryStore, format_rows, parse_since
from ccvoice.ports import DictationRecord


def add(store, clean, raw="", *, when="2026-03-01T10:00:00+00:00", model="whisper-base.en",
        app="Notes", seconds=5.0, method="fake"):
    return store.record(
        DictationRecord(
            timestamp=when, duration_s=seconds, model_id=model, target_app=app,
            raw_text=raw or clean, clean_text=clean, injection_method=method,
        )
    )


def test_a_dictation_is_stored_with_both_transcripts(harness, store):
    h = harness(history=store, text="um hello world")
    outcome = h.speak(seconds=2.0)

    entry = store.get(outcome.record_id)
    assert entry.raw_text == "um hello world"
    assert entry.clean_text == "Hello world."
    assert entry.word_count == 2
    assert entry.duration_s == 2.0
    assert entry.model_id == "fake-model"
    assert entry.target_app == "TestApp"
    assert entry.timestamp  # stamped automatically


def test_search_matches_either_transcript(store):
    add(store, "Ship the release", raw="um ship the release")
    add(store, "Buy milk")

    assert [r.clean_text for r in store.search("ship")] == ["Ship the release"]
    assert [r.clean_text for r in store.search("um")] == ["Ship the release"]
    assert store.search("nothing here") == []


def test_search_is_case_insensitive_and_ands_its_terms(store):
    add(store, "Deploy the API gateway")
    add(store, "Deploy the docs")

    assert len(store.search("deploy")) == 2
    assert len(store.search("DEPLOY API")) == 1


def test_search_filters_by_date_model_and_app(store):
    add(store, "old one", when="2026-01-01T09:00:00+00:00", model="whisper-tiny.en", app="Slack")
    add(store, "new one", when="2026-06-01T09:00:00+00:00", model="whisper-base.en", app="Notes")

    assert [r.clean_text for r in store.search(since="2026-05-01")] == ["new one"]
    assert [r.clean_text for r in store.search(until="2026-05-01")] == ["old one"]
    assert [r.clean_text for r in store.search(model_id="whisper-tiny.en")] == ["old one"]
    assert [r.clean_text for r in store.search(app="notes")] == ["new one"]


def test_results_are_newest_first_and_limited(store):
    for i in range(5):
        add(store, f"entry {i}", when=f"2026-03-0{i + 1}T10:00:00+00:00")

    rows = store.search(limit=2)
    assert [r.clean_text for r in rows] == ["entry 4", "entry 3"]


def test_history_survives_a_restart(home):
    store = HistoryStore(home / "history.db")
    add(store, "persist me")
    store.close()

    reopened = HistoryStore(home / "history.db")
    assert [r.clean_text for r in reopened.recent()] == ["persist me"]
    assert reopened.count() == 1
    reopened.close()


def test_delete_and_clear(store):
    first = add(store, "one")
    add(store, "two")
    assert store.delete(first) is True
    assert store.delete(first) is False
    assert store.count() == 1
    assert store.clear() == 1
    assert store.count() == 0


def test_statistics_aggregate_across_sessions(store):
    add(store, "one two three", seconds=6.0, when="2026-03-01T10:00:00+00:00")
    add(store, "four five", seconds=6.0, when="2026-03-01T11:00:00+00:00", app="Slack")
    add(store, "six", seconds=12.0, when="2026-03-02T10:00:00+00:00", model="whisper-tiny.en",
        method="clipboard")

    stats = store.stats()

    assert stats.sessions == 3
    assert stats.words == 6
    assert stats.seconds == 24.0
    assert stats.words_per_session == 2.0
    assert stats.words_per_minute == 15.0
    assert stats.longest_s == 12.0
    assert stats.busiest_day == ("2026-03-01", 5)
    assert stats.by_model == {"whisper-base.en": 5, "whisper-tiny.en": 1}
    assert stats.by_app == {"Notes": 4, "Slack": 2}
    assert stats.clipboard_fallbacks == 1
    assert "words/minute" in stats.render()


def test_statistics_can_be_scoped_to_a_window(store):
    add(store, "old", when="2026-01-01T10:00:00+00:00")
    add(store, "new words here", when="2026-06-01T10:00:00+00:00")

    stats = store.stats(since="2026-05-01")

    assert stats.sessions == 1 and stats.words == 3


def test_statistics_persist_across_restarts(home):
    store = HistoryStore(home / "history.db")
    add(store, "one two", seconds=4.0)
    before = store.stats()
    store.close()

    after = HistoryStore(home / "history.db").stats()
    assert (after.sessions, after.words, after.seconds) == (before.sessions, before.words, before.seconds)


def test_empty_statistics_render_a_friendly_line(store):
    assert store.stats().render() == "No dictations recorded yet."


def test_export_writes_one_json_object_per_line(store, tmp_path):
    add(store, "one")
    add(store, "two")
    target = tmp_path / "out.jsonl"

    assert store.export_jsonl(target) == 2
    lines = target.read_text(encoding="utf-8").strip().splitlines()
    assert len(lines) == 2 and lines[0].startswith("{")


def test_parse_since_accepts_relative_and_absolute():
    assert parse_since("2026-01-01") == "2026-01-01"
    assert parse_since("") == ""
    assert parse_since("7d") > "2020"
    assert parse_since("12h") > "2020"


def test_format_rows_is_readable_and_truncates(store):
    add(store, "a very long dictation " * 10)
    rendered = format_rows(store.recent())
    assert "..." in rendered
    assert format_rows([]) == "(no matches)"
