"""Local transcription, with a check that nothing goes over the network."""

import socket

import pytest

from ccvoice.netguard import NetworkBlocked, NetworkGuard
from ccvoice.ports import AudioClip
from ccvoice.stt import EchoEngine, clean_engine_output


def test_a_whole_dictation_runs_with_the_network_guard_armed(harness, guard):
    h = harness(guard=guard)
    outcome = h.speak(seconds=2.0)

    assert outcome.text == "Hello world."
    assert h.primary.injected == ["Hello world."]
    assert guard.attempts == []  # nothing even tried to connect
    assert not guard.armed  # and the guard was released afterwards


def test_the_guard_actually_blocks_a_connection(guard):
    with guard.offline():
        with pytest.raises(NetworkBlocked):
            socket.create_connection(("example.com", 80), timeout=1)
        with pytest.raises(NetworkBlocked):
            socket.socket().connect(("1.2.3.4", 443))
    assert guard.attempts == ["example.com", "1.2.3.4"]


def test_the_guard_restores_the_socket_module():
    original_socket, original_connect = socket.socket, socket.create_connection
    guard = NetworkGuard()
    with guard.offline():
        assert socket.socket is not original_socket
    assert socket.socket is original_socket
    assert socket.create_connection is original_connect


def test_nested_arming_only_unpatches_at_the_outermost_exit():
    original = socket.socket
    guard = NetworkGuard()
    guard.arm()
    guard.arm()
    guard.disarm()
    assert socket.socket is not original
    guard.disarm()
    assert socket.socket is original


def test_loopback_can_be_allowed_for_a_local_engine_server():
    guard = NetworkGuard(allow_loopback=True)
    with guard.offline():
        with pytest.raises(NetworkBlocked):
            socket.socket().connect(("8.8.8.8", 53))
        guard._check(("127.0.0.1", 1234))  # no exception


def test_the_engine_receives_the_recorded_audio(harness):
    h = harness()
    h.speak(seconds=3.0)
    duration, partial = h.engine.calls[-1]
    assert duration == pytest.approx(3.0, abs=0.01)
    assert partial is False


def test_transcription_survives_a_wav_round_trip(tmp_path):
    clip = AudioClip(b"\x01\x02" * 16_000)
    path = tmp_path / "clip.wav"
    clip.write_wav(path)
    reloaded = AudioClip.from_wav(path)

    assert reloaded.pcm == clip.pcm
    assert reloaded.sample_rate == 16_000
    assert reloaded.duration_s == pytest.approx(1.0)
    assert EchoEngine("ok").transcribe(reloaded).text == "ok"


def test_whisper_cpp_output_is_stripped_of_timestamps():
    raw = "[00:00:00.000 --> 00:00:02.000]   Hello there\n\n[00:00:02.000 --> 00:00:03.000]  friend\n"
    assert clean_engine_output(raw) == "Hello there friend"


def test_engine_output_without_timestamps_is_left_alone():
    assert clean_engine_output("  just text  \n") == "just text"
