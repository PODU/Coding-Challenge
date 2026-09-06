"""The command line: every subcommand a user can reach without a microphone."""

import pytest

from ccvoice.cli import main
from ccvoice.config import Config
from ccvoice.history import HistoryStore
from ccvoice.ports import DictationRecord


def run(args, home):
    return main(["--home", str(home), *args])


def test_format_runs_the_whole_pipeline(home, capsys):
    assert run(["format", "um", "first", "milk", "second", "eggs"], home) == 0
    assert capsys.readouterr().out.strip() == "1. Milk\n2. Eggs"


def test_format_explain_shows_each_stage(home, capsys):
    assert run(["format", "--explain", "um hello new line world"], home) == 0
    out = capsys.readouterr().out
    assert "commands" in out and "fillers" in out and "final" in out


def test_commands_lists_the_spoken_vocabulary(home, capsys):
    assert run(["commands"], home) == 0
    out = capsys.readouterr().out
    assert "new line" in out and "literally" in out and "bullet point" in out


def test_dict_add_list_and_remove(home, capsys):
    assert run(["dict", "add", "clod code", "Claude", "Code"], home) == 0
    assert run(["dict", "list"], home) == 0
    assert "clod code" in capsys.readouterr().out
    assert Config.load(home).dictionary == {"clod code": "Claude Code"}

    assert run(["dict", "rm", "clod code"], home) == 0
    assert Config.load(home).dictionary == {}


def test_the_dictionary_is_used_by_format(home, capsys):
    run(["dict", "add", "github", "GitHub"], home)
    capsys.readouterr()
    run(["format", "push to github"], home)
    assert capsys.readouterr().out.strip() == "Push to GitHub."


def test_models_list_marks_the_active_model(home, capsys):
    assert run(["models", "list"], home) == 0
    out = capsys.readouterr().out
    assert "whisper-base.en" in out and "*active*" in out
    assert "not downloaded" in out


def test_models_use_rejects_an_unknown_id(home, capsys):
    assert run(["models", "use", "whisper-enormous"], home) == 2
    assert "unknown model" in capsys.readouterr().err


def test_history_search_show_and_export(home, capsys, tmp_path):
    store = HistoryStore(home / "history.db")
    store.record(DictationRecord(clean_text="ship the release", raw_text="um ship the release",
                                 duration_s=3.0, model_id="whisper-base.en", target_app="Notes"))
    store.close()

    assert run(["history", "search", "ship"], home) == 0
    assert "ship the release" in capsys.readouterr().out

    assert run(["history", "show", "1"], home) == 0
    shown = capsys.readouterr().out
    assert "um ship the release" in shown and "whisper-base.en" in shown

    target = tmp_path / "export.jsonl"
    assert run(["history", "export", str(target)], home) == 0
    assert target.read_text(encoding="utf-8").count("\n") == 1

    assert run(["history", "clear"], home) == 0
    assert run(["history", "show", "1"], home) == 1


def test_history_search_honours_since(home, capsys):
    store = HistoryStore(home / "history.db")
    store.record(DictationRecord(timestamp="2020-01-01T00:00:00+00:00", clean_text="ancient"))
    store.close()

    run(["history", "search", "--since", "7d"], home)
    assert "(no matches)" in capsys.readouterr().out


def test_stats_reports_the_aggregate(home, capsys):
    store = HistoryStore(home / "history.db")
    store.record(DictationRecord(clean_text="one two three", duration_s=6.0))
    store.close()

    assert run(["stats"], home) == 0
    out = capsys.readouterr().out
    assert "sessions          1" in out and "words             3" in out


def test_stats_on_an_empty_history(home, capsys):
    assert run(["stats"], home) == 0
    assert "No dictations recorded yet." in capsys.readouterr().out


def test_doctor_reports_every_subsystem(home, capsys):
    assert run(["doctor"], home) == 0
    out = capsys.readouterr().out
    for expected in ("audio capture", "hotkey backend", "text injection", "clipboard",
                     "whisper binary", "models", "perm microphone", "perm input_injection", "history"):
        assert expected in out


def test_transcribe_needs_a_downloaded_model(home, tmp_path, capsys):
    from ccvoice.ports import AudioClip

    wav = tmp_path / "clip.wav"
    AudioClip.silence(1.0).write_wav(wav)

    assert run(["transcribe", str(wav)], home) == 2
    assert "not downloaded" in capsys.readouterr().err


def test_unknown_command_is_rejected(home):
    with pytest.raises(SystemExit):
        run(["frobnicate"], home)
