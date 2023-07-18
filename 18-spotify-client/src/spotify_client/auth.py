"""OAuth 2.0 Authorization Code flow with PKCE.

``login`` opens the browser and runs a short-lived local HTTP server to catch
the redirect. The other pieces are separate functions so they can be tested
without a browser or network.
"""

from __future__ import annotations

import base64
import hashlib
import secrets
import threading
import urllib.parse
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Optional

import requests

from .config import AUTHORIZE_URL, TOKEN_URL, Config
from .errors import AuthError
from .tokenstore import Token, TokenStore

DEFAULT_TIMEOUT = 15


def generate_code_verifier(length: int = 64) -> str:
    """PKCE requires the verifier to be 43-128 URL-safe characters."""
    if not 43 <= length <= 128:
        raise ValueError("code_verifier length must be between 43 and 128")
    # 96 random bytes encode to 128 characters, enough for any valid length.
    return secrets.token_urlsafe(96)[:length]


def generate_code_challenge(verifier: str) -> str:
    """base64url(SHA256(verifier)) without padding, as S256 requires."""
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    return base64.urlsafe_b64encode(digest).decode("ascii").rstrip("=")


def generate_state(length: int = 16) -> str:
    return secrets.token_urlsafe(length)


class _CallbackHandler(BaseHTTPRequestHandler):
    """Stores the redirect's query parameters on ``server.oauth_result``."""

    server_version = "spotify-client/0.1"

    def do_GET(self) -> None:  # noqa: N802 (http.server naming)
        parsed = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qs(parsed.query)
        self.server.oauth_result = {k: v[0] for k, v in params.items()}  # type: ignore[attr-defined]

        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        if "error" in params:
            body = "<h1>Login failed</h1><p>You can close this tab.</p>"
        else:
            body = "<h1>Login complete</h1><p>You can close this tab and return to the terminal.</p>"
        self.wfile.write(body.encode("utf-8"))

    def log_message(self, *args: object) -> None:
        """Keep http.server from logging every request to stderr."""


def run_callback_server(redirect_uri: str, timeout: int = 300) -> dict:
    """Wait up to ``timeout`` seconds for the redirect and return its query
    parameters."""
    parsed = urllib.parse.urlparse(redirect_uri)
    host = parsed.hostname or "127.0.0.1"
    port = parsed.port or 8888
    try:
        httpd = HTTPServer((host, port), _CallbackHandler)
    except OSError as exc:
        raise AuthError(
            f"Could not bind {host}:{port} for the OAuth redirect ({exc}). "
            "Free the port or set a different SPOTIFY_REDIRECT_URI."
        ) from exc

    httpd.oauth_result = None  # type: ignore[attr-defined]
    httpd.timeout = timeout

    def _serve() -> None:
        # Requests without the redirect (e.g. favicon) leave oauth_result unset.
        while getattr(httpd, "oauth_result", None) is None:
            httpd.handle_request()

    thread = threading.Thread(target=_serve, daemon=True)
    thread.start()
    thread.join(timeout)
    result = getattr(httpd, "oauth_result", None)
    httpd.server_close()
    if result is None:
        raise AuthError("Timed out waiting for the Spotify login redirect.")
    return result


def parse_callback(params: dict, expected_state: str) -> str:
    """Check the redirect parameters and return the authorization code."""
    if "error" in params:
        raise AuthError(f"Authorization denied or failed: {params['error']}.")
    if params.get("state") != expected_state:
        raise AuthError("OAuth state mismatch - possible CSRF; aborting login.")
    code = params.get("code")
    if not code:
        raise AuthError("No authorization code returned by Spotify.")
    return code


class AuthManager:
    """Handles login and keeps the access token fresh."""

    def __init__(self, config: Config, store: TokenStore,
                 session: Optional[requests.Session] = None) -> None:
        self.config = config
        self.store = store
        self.session = session or requests.Session()
        self._token: Optional[Token] = store.load()

    def authorize_url(self, code_challenge: str, state: str) -> str:
        params = {
            "response_type": "code",
            "client_id": self.config.client_id,
            "redirect_uri": self.config.redirect_uri,
            "scope": self.config.scope_string,
            "code_challenge_method": "S256",
            "code_challenge": code_challenge,
            "state": state,
        }
        return f"{AUTHORIZE_URL}?{urllib.parse.urlencode(params)}"

    def exchange_code(self, code: str, code_verifier: str) -> Token:
        data = {
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": self.config.redirect_uri,
            "client_id": self.config.client_id,
            "code_verifier": code_verifier,
        }
        token = self._post_token(data)
        self._token = token
        self.store.save(token)
        return token

    def refresh(self) -> Token:
        if not self._token or not self._token.refresh_token:
            raise AuthError("No refresh token available; run `spotify login`.")
        data = {
            "grant_type": "refresh_token",
            "refresh_token": self._token.refresh_token,
            "client_id": self.config.client_id,
        }
        token = self._post_token(data, previous=self._token)
        self._token = token
        self.store.save(token)
        return token

    def _post_token(self, data: dict, previous: Optional[Token] = None) -> Token:
        try:
            resp = self.session.post(TOKEN_URL, data=data, timeout=DEFAULT_TIMEOUT)
        except requests.RequestException as exc:
            raise AuthError(f"Network error contacting Spotify token endpoint: {exc}") from exc
        if resp.status_code != 200:
            raise AuthError(
                f"Token request failed ({resp.status_code}): {_safe_error_detail(resp)}"
            )
        return Token.from_response(resp.json(), previous=previous)

    def valid_access_token(self) -> str:
        """Return an access token, refreshing it first if it has expired."""
        if self._token is None:
            raise AuthError("Not logged in. Run `spotify login` first.")
        if self._token.is_expired():
            self.refresh()
        assert self._token is not None
        return self._token.access_token

    @property
    def token(self) -> Optional[Token]:
        return self._token

    def login(self, *, open_browser: bool = True) -> Token:
        verifier = generate_code_verifier()
        challenge = generate_code_challenge(verifier)
        state = generate_state()
        url = self.authorize_url(challenge, state)

        print("Opening browser for Spotify authorization...")
        print(f"If it does not open, visit:\n{url}\n")
        if open_browser:
            try:
                webbrowser.open(url)
            except Exception:  # pragma: no cover - headless environments
                pass

        params = run_callback_server(self.config.redirect_uri)
        code = parse_callback(params, state)
        return self.exchange_code(code, verifier)


def _safe_error_detail(resp: requests.Response) -> str:
    try:
        body = resp.json()
    except ValueError:
        return resp.text[:200]
    return body.get("error_description") or body.get("error") or str(body)


__all__ = [
    "AuthManager",
    "generate_code_verifier",
    "generate_code_challenge",
    "generate_state",
    "run_callback_server",
    "parse_callback",
]
