"""cczipcrack: an educational ZIP password cracker.

Recovers a ZIP password by dictionary or brute-force attack, checking each
candidate with a real decrypt and CRC check. Handles legacy ZipCrypto, and
WinZip AES when ``pyzipper`` is installed.

Only use this on archives you own or are authorized to test.
"""

from __future__ import annotations

__all__ = ["__version__"]
__version__ = "1.0.0"
