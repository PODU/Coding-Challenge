"""Exceptions. Each one maps to a CLI message telling the user what to do."""

from __future__ import annotations

from typing import Optional


class SpotifyError(Exception):
    """Base class for this package's errors."""


class ConfigError(SpotifyError):
    """Required configuration (e.g. ``SPOTIFY_CLIENT_ID``) is missing/invalid."""


class AuthError(SpotifyError):
    """Not logged in, or the login is no longer valid."""


class ApiError(SpotifyError):
    """Any other failed API call."""

    def __init__(self, message: str, status_code: Optional[int] = None) -> None:
        super().__init__(message)
        self.status_code = status_code


class NoActiveDeviceError(SpotifyError):
    """No device is available to take playback commands."""


class PremiumRequiredError(SpotifyError):
    """Playback control needs Spotify Premium."""


class RateLimited(SpotifyError):
    """Still getting 429 after one retry."""

    def __init__(self, message: str, retry_after: Optional[int] = None) -> None:
        super().__init__(message)
        self.retry_after = retry_after


__all__ = [
    "SpotifyError",
    "ConfigError",
    "AuthError",
    "ApiError",
    "NoActiveDeviceError",
    "PremiumRequiredError",
    "RateLimited",
]
