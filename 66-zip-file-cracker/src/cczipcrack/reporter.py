"""The exact output strings, kept in one place so the CLI wording stays
consistent."""

from __future__ import annotations

HAS_HEADERS = "Has zip headers"
NO_HEADERS = "Does not have zip headers"
NOT_ENCRYPTED = "Archive is not encrypted; nothing to crack"
AES_UNSUPPORTED = "AES-encrypted; install pyzipper to crack."
PASSWORD_NOT_FOUND = "Password not found"


def has_headers() -> str:
    return HAS_HEADERS


def no_headers() -> str:
    return NO_HEADERS


def password_found(password: str) -> str:
    return f"Password found: {password}"


def password_not_found() -> str:
    return PASSWORD_NOT_FOUND


def not_encrypted() -> str:
    return NOT_ENCRYPTED


def aes_unsupported() -> str:
    return AES_UNSUPPORTED
