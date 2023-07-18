"""Tests for env-var config loading and token persistence."""

from __future__ import annotations

import os
import time

import pytest

from spotify_client.config import DEFAULT_REDIRECT_URI, Config
from spotify_client.errors import ConfigError
from spotify_client.tokenstore import Token, TokenStore


def test_config_from_env_requires_client_id():
    with pytest.raises(ConfigError):
        Config.from_env(env={})


def test_config_from_env_defaults_redirect():
    cfg = Config.from_env(env={"SPOTIFY_CLIENT_ID": "abc"})
    assert cfg.client_id == "abc"
    assert cfg.redirect_uri == DEFAULT_REDIRECT_URI
    assert "playlist-read-private" in cfg.scope_string


def test_config_from_env_custom_redirect():
    cfg = Config.from_env(env={
        "SPOTIFY_CLIENT_ID": "abc",
        "SPOTIFY_REDIRECT_URI": "http://127.0.0.1:9000/cb",
    })
    assert cfg.redirect_uri == "http://127.0.0.1:9000/cb"


def test_token_round_trip(tmp_path):
    store = TokenStore(path=tmp_path / "t.json")
    token = Token(access_token="a", refresh_token="r",
                  expires_at=time.time() + 100, scope="s")
    store.save(token)
    loaded = store.load()
    assert loaded.access_token == "a"
    assert loaded.refresh_token == "r"
    assert loaded.scope == "s"


def test_token_store_missing_returns_none(tmp_path):
    assert TokenStore(path=tmp_path / "none.json").load() is None


def test_token_store_corrupt_returns_none(tmp_path):
    p = tmp_path / "bad.json"
    p.write_text("not json", encoding="utf-8")
    assert TokenStore(path=p).load() is None


def test_token_store_clear(tmp_path):
    store = TokenStore(path=tmp_path / "t.json")
    store.save(Token(access_token="a", expires_at=time.time() + 100))
    store.clear()
    assert store.load() is None
    store.clear()  # idempotent


@pytest.mark.skipif(os.name == "nt", reason="POSIX permission bits only")
def test_token_saved_0600(tmp_path):
    store = TokenStore(path=tmp_path / "t.json")
    store.save(Token(access_token="a", expires_at=time.time() + 100))
    mode = (tmp_path / "t.json").stat().st_mode & 0o777
    assert mode == 0o600


def test_token_from_response_carries_refresh_forward():
    prev = Token(access_token="old", refresh_token="keep",
                 expires_at=0, scope="sc")
    new = Token.from_response(
        {"access_token": "new", "expires_in": 3600}, previous=prev)
    assert new.refresh_token == "keep"
    assert new.scope == "sc"
