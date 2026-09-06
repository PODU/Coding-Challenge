"""Multiple models, downloaded on demand and switchable at runtime."""

import hashlib
import io
import zipfile

import pytest

from ccvoice.models import CATALOG, DownloadProgress, FetchResponse, ModelError, ModelManager
from ccvoice.stt import EchoEngine


def fetcher_for(payload: bytes, *, honour_range: bool = True, chunk: int = 8):
    """A stand-in for HTTP: yields `payload` in small pieces, optionally resuming."""
    calls = []

    def fetch(url, offset=0):
        calls.append((url, offset))
        body = payload[offset:] if honour_range else payload
        chunks = (body[i : i + chunk] for i in range(0, len(body), chunk))
        return FetchResponse(chunks, len(payload), resumed=bool(offset) and honour_range)

    fetch.calls = calls
    return fetch


def test_catalogue_offers_a_speed_accuracy_spread():
    assert len(CATALOG) >= 2
    engines = {spec.engine for spec in CATALOG.values()}
    assert {"whisper.cpp", "vosk"} <= engines
    assert all(spec.speed and spec.accuracy for spec in CATALOG.values())
    sizes = [spec.size_bytes for spec in CATALOG.values()]
    assert min(sizes) * 3 < max(sizes)  # a genuinely different trade-off, not four of the same


def test_unknown_model_id_is_a_clear_error():
    with pytest.raises(ModelError) as excinfo:
        ModelManager.spec("whisper-enormous")
    assert "unknown model" in str(excinfo.value)


def test_download_reports_progress_and_lands_the_file(home):
    payload = b"weights" * 40
    manager = ModelManager(home, fetcher_for(payload))
    updates: list[DownloadProgress] = []

    path = manager.ensure("whisper-tiny.en", updates.append)

    assert path.read_bytes() == payload
    assert manager.is_installed("whisper-tiny.en")
    assert updates[-1].done and updates[-1].fraction == 1.0
    assert [u.downloaded for u in updates] == sorted(u.downloaded for u in updates)
    assert "%" in updates[0].bar()


def test_an_installed_model_is_not_downloaded_again(home):
    fetch = fetcher_for(b"weights")
    manager = ModelManager(home, fetch)
    manager.ensure("whisper-tiny.en")
    manager.ensure("whisper-tiny.en")
    assert len(fetch.calls) == 1


def test_a_partial_download_resumes_from_where_it_stopped(home):
    payload = b"0123456789" * 10
    manager = ModelManager(home, fetcher_for(payload))
    part = manager.root
    part.mkdir(parents=True, exist_ok=True)
    (part / (ModelManager.spec("whisper-tiny.en").filename + ".part")).write_bytes(payload[:40])

    manager.ensure("whisper-tiny.en")

    assert manager.path("whisper-tiny.en").read_bytes() == payload
    assert manager.fetcher.calls == [(ModelManager.spec("whisper-tiny.en").url, 40)]


def test_a_server_that_ignores_range_restarts_cleanly(home):
    payload = b"abcdefghij" * 5
    manager = ModelManager(home, fetcher_for(payload, honour_range=False))
    manager.root.mkdir(parents=True, exist_ok=True)
    (manager.root / (ModelManager.spec("whisper-tiny.en").filename + ".part")).write_bytes(b"junk")

    manager.ensure("whisper-tiny.en")

    assert manager.path("whisper-tiny.en").read_bytes() == payload


def test_a_failed_download_leaves_no_usable_model_behind(home, monkeypatch):
    payload = b"corrupt"
    spec = ModelManager.spec("whisper-tiny.en")
    monkeypatch.setitem(CATALOG, spec.id, type(spec)(**{**spec.__dict__, "sha256": "0" * 64}))
    manager = ModelManager(home, fetcher_for(payload))

    with pytest.raises(ModelError) as excinfo:
        manager.ensure("whisper-tiny.en")

    assert "checksum mismatch" in str(excinfo.value)
    assert not manager.is_installed("whisper-tiny.en")
    assert not (manager.root / (spec.filename + ".part")).exists()


def test_a_correct_checksum_passes(home, monkeypatch):
    payload = b"good weights"
    spec = ModelManager.spec("whisper-tiny.en")
    digest = hashlib.sha256(payload).hexdigest()
    monkeypatch.setitem(CATALOG, spec.id, type(spec)(**{**spec.__dict__, "sha256": digest}))
    manager = ModelManager(home, fetcher_for(payload))

    assert manager.ensure("whisper-tiny.en").read_bytes() == payload


def test_an_archive_model_is_expanded_into_a_directory(home):
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr("vosk-model-small-en-us-0.15/am/final.mdl", "weights")
        zf.writestr("vosk-model-small-en-us-0.15/conf/model.conf", "conf")
    manager = ModelManager(home, fetcher_for(buf.getvalue()))

    path = manager.ensure("vosk-small-en-us")

    assert path.is_dir()
    assert (path / "am" / "final.mdl").read_text() == "weights"
    assert manager.is_installed("vosk-small-en-us")
    assert not (manager.root / "vosk-model-small-en-us-0.15.part").exists()


def test_installed_and_describe_report_state(home):
    manager = ModelManager(home, fetcher_for(b"x"))
    assert manager.installed() == []
    manager.ensure("whisper-tiny.en")
    assert manager.installed() == ["whisper-tiny.en"]
    described = dict((mid, ok) for mid, _spec, ok in manager.describe())
    assert described["whisper-tiny.en"] is True
    assert described["whisper-base.en"] is False


def test_remove_deletes_the_weights(home):
    manager = ModelManager(home, fetcher_for(b"x"))
    manager.ensure("whisper-tiny.en")
    assert manager.remove("whisper-tiny.en") is True
    assert manager.remove("whisper-tiny.en") is False


def test_switching_model_is_instant_once_downloaded(harness):
    h = harness()
    h.speak(seconds=1.0)
    assert h.app.outcomes[-1].record_id is None

    h.app.switch_model(EchoEngine("second model speaking", model_id="whisper-small.en"))
    outcome = h.speak(seconds=1.0)

    assert outcome.text == "Second model speaking."
    assert h.app.config.model_id == "whisper-small.en"
    assert h.indicator.notifications[-1] == "Model: whisper-small.en"
