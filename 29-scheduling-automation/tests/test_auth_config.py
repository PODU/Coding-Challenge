"""Tests for OAuth token refresh and environment configuration."""

import json

import pytest

from scheduler.config import Settings, parse_availability_spec
from scheduler.providers.auth import AuthError, OAuthToken


class _FakeResponse:
    def __init__(self, status_code=200, payload=None):
        self.status_code = status_code
        self._payload = payload or {}

    def json(self):
        return self._payload


class TestOAuthRefresh:
    def _token(self, expiry=0.0):
        return OAuthToken(
            access_token="old",
            refresh_token="refresh",
            token_uri="https://oauth.example/token",
            client_id="cid",
            client_secret="secret",
            expiry=expiry,
        )

    def test_refresh_updates_token(self):
        token = self._token()
        calls = {}

        def fake_post(url, data, timeout):
            calls["url"] = url
            calls["data"] = data
            return _FakeResponse(200, {"access_token": "new", "expires_in": 3600})

        new = token.refresh(http_post=fake_post, now=1000.0)
        assert new == "new"
        assert token.access_token == "new"
        assert token.expiry == 1000.0 + 3600
        assert calls["data"]["grant_type"] == "refresh_token"

    def test_refresh_rejected_raises_auth_error(self):
        token = self._token()

        def fake_post(url, data, timeout):
            return _FakeResponse(400)

        with pytest.raises(AuthError):
            token.refresh(http_post=fake_post)

    def test_authorization_header_refreshes_when_expired(self):
        token = self._token(expiry=0.0)  # already expired

        def fake_post(url, data, timeout):
            return _FakeResponse(200, {"access_token": "fresh", "expires_in": 3600})

        header = token.authorization_header(http_post=fake_post)
        assert header == "Bearer fresh"

    def test_from_json_roundtrip(self):
        raw = json.dumps(
            {
                "access_token": "a",
                "refresh_token": "r",
                "token_uri": "u",
                "client_id": "c",
                "client_secret": "s",
                "expiry": 5.0,
            }
        )
        token = OAuthToken.from_json(raw)
        assert token.access_token == "a"
        assert token.is_expired(now=100.0)


class TestSettings:
    def test_from_env_parses_calendars(self):
        env = {
            "CALDAV_URL": "https://dav",
            "CALDAV_USERNAME": "alice",
            "CALENDARS": "work, personal ,",
            "ORGANIZER_TZ": "Europe/London",
        }
        settings = Settings.from_env(env)
        assert settings.calendars == ("work", "personal")
        assert settings.organizer_tz == "Europe/London"


class TestAvailabilitySpec:
    def test_range_and_single_day(self):
        avail = parse_availability_spec("Mon-Fri 09:00-17:00, Sat 10:00-12:00", "UTC")
        weekdays = sorted(r.weekday for r in avail.rules)
        assert weekdays == [0, 1, 2, 3, 4, 5]

    def test_malformed_spec_raises(self):
        with pytest.raises(ValueError):
            parse_availability_spec("garbage", "UTC")
