import pytest

from reviewagent.retry import RetryableError, with_retries


def test_succeeds_first_try():
    assert with_retries(lambda: 42, sleep=lambda _: None) == 42


def test_retries_then_succeeds():
    calls = {"n": 0}

    def flaky():
        calls["n"] += 1
        if calls["n"] < 3:
            raise RetryableError("boom")
        return "ok"

    delays = []
    assert with_retries(flaky, attempts=3, sleep=delays.append) == "ok"
    assert calls["n"] == 3
    assert delays == [1.0, 2.0]  # exponential backoff


def test_exhausts_attempts():
    def always_fails():
        raise RetryableError("down")

    with pytest.raises(RetryableError):
        with_retries(always_fails, attempts=3, sleep=lambda _: None)


def test_non_retryable_propagates_immediately():
    calls = {"n": 0}

    def bad_auth():
        calls["n"] += 1
        raise ValueError("401 unauthorized")

    with pytest.raises(ValueError):
        with_retries(bad_auth, attempts=3, sleep=lambda _: None)
    assert calls["n"] == 1  # a 401 never fixes itself; don't retry


def test_retry_after_overrides_backoff():
    calls = {"n": 0}

    def rate_limited():
        calls["n"] += 1
        if calls["n"] == 1:
            raise RetryableError("429", retry_after=7.5)
        return "ok"

    delays = []
    assert with_retries(rate_limited, sleep=delays.append) == "ok"
    assert delays == [7.5]


def test_max_delay_cap():
    calls = {"n": 0}

    def flaky():
        calls["n"] += 1
        if calls["n"] < 5:
            raise RetryableError("boom")
        return "ok"

    delays = []
    with_retries(flaky, attempts=5, base_delay=10, max_delay=25,
                 sleep=delays.append)
    assert delays == [10, 20, 25, 25]
