"""Auth tests: client-credentials, auth-code+PKCE, refresh, caching."""

from __future__ import annotations

import base64
import hashlib
import json
from urllib.parse import parse_qs, urlparse

import pytest
import responses

from spotify_playlist.auth import (
    AuthorizationCodeTokenProvider,
    ClientCredentialsTokenProvider,
    TOKEN_URL,
    build_authorize_url,
    generate_pkce_pair,
)


@responses.activate
def test_client_credentials_basic_header(config):
    responses.post(
        TOKEN_URL,
        json={"access_token": "abc", "expires_in": 3600, "token_type": "Bearer"},
        status=200,
    )
    provider = ClientCredentialsTokenProvider(config)
    token = provider.get_token()
    assert token == "abc"

    sent = responses.calls[0].request
    expected = "Basic " + base64.b64encode(b"test-client-id:test-client-secret").decode()
    assert sent.headers["Authorization"] == expected
    assert "grant_type=client_credentials" in sent.body


@responses.activate
def test_client_credentials_caches_until_expiry(config):
    responses.post(TOKEN_URL, json={"access_token": "abc", "expires_in": 3600}, status=200)
    clock = [1000.0]
    provider = ClientCredentialsTokenProvider(config, time_func=lambda: clock[0])

    assert provider.get_token() == "abc"
    assert provider.get_token() == "abc"  # cached, no second HTTP call
    assert len(responses.calls) == 1


@responses.activate
def test_client_credentials_refreshes_after_expiry(config):
    responses.post(TOKEN_URL, json={"access_token": "t1", "expires_in": 3600}, status=200)
    responses.post(TOKEN_URL, json={"access_token": "t2", "expires_in": 3600}, status=200)
    clock = [1000.0]
    provider = ClientCredentialsTokenProvider(config, time_func=lambda: clock[0])

    assert provider.get_token() == "t1"
    clock[0] += 4000  # well past expiry
    assert provider.get_token() == "t2"
    assert len(responses.calls) == 2


def test_generate_pkce_pair_is_s256():
    verifier, challenge = generate_pkce_pair()
    expected = (
        base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest())
        .decode()
        .rstrip("=")
    )
    assert challenge == expected
    assert "=" not in challenge  # base64url, no padding


def test_build_authorize_url_contains_pkce_and_scopes(config):
    url = build_authorize_url(
        config, state="xyz", code_challenge="CHALLENGE", scopes=["a", "b"]
    )
    qs = parse_qs(urlparse(url).query)
    assert url.startswith("https://accounts.spotify.com/authorize?")
    assert qs["response_type"] == ["code"]
    assert qs["client_id"] == ["test-client-id"]
    assert qs["redirect_uri"] == ["http://127.0.0.1:8888/callback"]
    assert qs["code_challenge_method"] == ["S256"]
    assert qs["code_challenge"] == ["CHALLENGE"]
    assert qs["state"] == ["xyz"]
    assert qs["scope"] == ["a b"]


@responses.activate
def test_authorization_code_exchange_and_cache(config):
    responses.post(
        TOKEN_URL,
        json={"access_token": "user-token", "refresh_token": "refresh-1", "expires_in": 3600},
        status=200,
    )
    provider = AuthorizationCodeTokenProvider(
        config, authorizer=lambda url: "the-code"
    )
    assert provider.get_token() == "user-token"

    # Exchange used grant_type=authorization_code + PKCE verifier.
    body = parse_qs(responses.calls[0].request.body)
    assert body["grant_type"] == ["authorization_code"]
    assert body["code"] == ["the-code"]
    assert "code_verifier" in body

    # Refresh token persisted to the 0600 cache file.
    cached = json.loads(config.token_cache_path.read_text())
    assert cached["refresh_token"] == "refresh-1"


@responses.activate
def test_authorization_code_refresh_on_expiry(config):
    # Pre-seed a cache with only a refresh token (expired access token).
    config.token_cache_path.write_text(
        json.dumps({"refresh_token": "refresh-1", "access_token": "old", "expires_at": 0})
    )
    responses.post(
        TOKEN_URL,
        json={"access_token": "fresh-token", "expires_in": 3600},
        status=200,
    )
    provider = AuthorizationCodeTokenProvider(config)
    # No authorizer needed: it must use the refresh token.
    assert provider.get_token() == "fresh-token"
    body = parse_qs(responses.calls[0].request.body)
    assert body["grant_type"] == ["refresh_token"]
    assert body["refresh_token"] == ["refresh-1"]


@responses.activate
def test_refresh_token_rotation_is_persisted(config):
    config.token_cache_path.write_text(
        json.dumps({"refresh_token": "old-refresh", "expires_at": 0})
    )
    responses.post(
        TOKEN_URL,
        json={"access_token": "t", "refresh_token": "new-refresh", "expires_in": 3600},
        status=200,
    )
    provider = AuthorizationCodeTokenProvider(config)
    provider.get_token()
    cached = json.loads(config.token_cache_path.read_text())
    assert cached["refresh_token"] == "new-refresh"
