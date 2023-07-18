"""Tests for PKCE generation, authorize URL, token exchange and refresh."""

from __future__ import annotations

import base64
import hashlib
import time
import urllib.parse

import pytest
import responses

from spotify_client.auth import (
    AuthManager,
    generate_code_challenge,
    generate_code_verifier,
    generate_state,
    parse_callback,
)
from spotify_client.config import TOKEN_URL
from spotify_client.errors import AuthError


def test_code_verifier_length_bounds():
    v = generate_code_verifier(64)
    assert 43 <= len(v) <= 128
    with pytest.raises(ValueError):
        generate_code_verifier(10)
    with pytest.raises(ValueError):
        generate_code_verifier(200)


def test_code_challenge_is_base64url_sha256_no_padding():
    verifier = "a" * 64
    expected = (
        base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest())
        .decode()
        .rstrip("=")
    )
    challenge = generate_code_challenge(verifier)
    assert challenge == expected
    assert "=" not in challenge
    assert "+" not in challenge and "/" not in challenge


def test_authorize_url_contains_required_params(config, token_store):
    mgr = AuthManager(config, token_store)
    url = mgr.authorize_url("CHALLENGE", "STATE")
    parsed = urllib.parse.urlparse(url)
    qs = urllib.parse.parse_qs(parsed.query)
    assert qs["response_type"] == ["code"]
    assert qs["client_id"] == ["test-client-id"]
    assert qs["code_challenge"] == ["CHALLENGE"]
    assert qs["code_challenge_method"] == ["S256"]
    assert qs["state"] == ["STATE"]
    assert qs["redirect_uri"] == ["http://127.0.0.1:8888/callback"]
    # All required scopes present.
    scopes = qs["scope"][0].split()
    for required in ("playlist-read-private", "user-modify-playback-state",
                     "user-read-playback-state"):
        assert required in scopes


@responses.activate
def test_exchange_code_parses_tokens_and_expiry(config, token_store):
    responses.add(
        responses.POST, TOKEN_URL,
        json={
            "access_token": "new-access",
            "refresh_token": "new-refresh",
            "expires_in": 3600,
            "token_type": "Bearer",
            "scope": "user-read-private",
        },
        status=200,
    )
    mgr = AuthManager(config, token_store)
    before = time.time()
    token = mgr.exchange_code("auth-code", "verifier")
    assert token.access_token == "new-access"
    assert token.refresh_token == "new-refresh"
    assert token.expires_at >= before + 3600 - 5
    # Persisted to the store.
    assert token_store.load().access_token == "new-access"
    # Request body carried the PKCE verifier and code.
    body = urllib.parse.parse_qs(responses.calls[0].request.body)
    assert body["grant_type"] == ["authorization_code"]
    assert body["code_verifier"] == ["verifier"]
    assert body["code"] == ["auth-code"]


@responses.activate
def test_refresh_uses_refresh_token_and_keeps_old_when_absent(
    config, token_store, expired_token
):
    token_store.save(expired_token)
    responses.add(
        responses.POST, TOKEN_URL,
        json={"access_token": "refreshed-access", "expires_in": 3600,
              "token_type": "Bearer"},
        status=200,
    )
    mgr = AuthManager(config, token_store)
    # valid_access_token should detect expiry and refresh transparently.
    access = mgr.valid_access_token()
    assert access == "refreshed-access"
    # No refresh_token in response => keep the previous one.
    assert mgr.token.refresh_token == "refresh-123"
    body = urllib.parse.parse_qs(responses.calls[0].request.body)
    assert body["grant_type"] == ["refresh_token"]
    assert body["refresh_token"] == ["refresh-123"]


@responses.activate
def test_refresh_failure_raises_auth_error(config, token_store, expired_token):
    token_store.save(expired_token)
    responses.add(
        responses.POST, TOKEN_URL,
        json={"error": "invalid_grant",
              "error_description": "Refresh token revoked"},
        status=400,
    )
    mgr = AuthManager(config, token_store)
    with pytest.raises(AuthError) as exc:
        mgr.valid_access_token()
    assert "Refresh token revoked" in str(exc.value)


def test_valid_access_token_without_login_raises(config, token_store):
    mgr = AuthManager(config, token_store)  # empty store
    with pytest.raises(AuthError):
        mgr.valid_access_token()


def test_parse_callback_state_mismatch():
    with pytest.raises(AuthError):
        parse_callback({"code": "x", "state": "WRONG"}, "EXPECTED")


def test_parse_callback_access_denied():
    with pytest.raises(AuthError) as exc:
        parse_callback({"error": "access_denied", "state": "S"}, "S")
    assert "denied" in str(exc.value).lower()


def test_parse_callback_success_returns_code():
    assert parse_callback({"code": "the-code", "state": "S"}, "S") == "the-code"


def test_generate_state_is_random():
    assert generate_state() != generate_state()
