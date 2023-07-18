"""Shared pytest fixtures. All HTTP is mocked; no real credentials are used."""

from __future__ import annotations

import time

import pytest
import requests

from spotify_client.auth import AuthManager
from spotify_client.api import SpotifyClient
from spotify_client.config import Config
from spotify_client.tokenstore import Token, TokenStore


@pytest.fixture
def config() -> Config:
    """A deterministic config that never touches the real environment."""
    return Config(
        client_id="test-client-id",
        redirect_uri="http://127.0.0.1:8888/callback",
    )


@pytest.fixture
def token_store(tmp_path) -> TokenStore:
    """A TokenStore pointed at an isolated temp file."""
    return TokenStore(path=tmp_path / "tokens.json")


@pytest.fixture
def valid_token() -> Token:
    """A token that is valid for the next hour."""
    return Token(
        access_token="access-123",
        refresh_token="refresh-123",
        expires_at=time.time() + 3600,
        scope="user-read-private",
    )


@pytest.fixture
def expired_token() -> Token:
    """A token that is already expired."""
    return Token(
        access_token="old-access",
        refresh_token="refresh-123",
        expires_at=time.time() - 10,
        scope="user-read-private",
    )


@pytest.fixture
def auth(config, token_store, valid_token) -> AuthManager:
    """An AuthManager pre-seeded with a valid token in its store."""
    token_store.save(valid_token)
    return AuthManager(config, token_store, session=requests.Session())


@pytest.fixture
def client(auth) -> SpotifyClient:
    """A SpotifyClient backed by a logged-in AuthManager. Sleep is a no-op."""
    return SpotifyClient(auth, session=auth.session, sleep=lambda _s: None)
