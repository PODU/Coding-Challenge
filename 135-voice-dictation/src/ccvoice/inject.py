"""Type the text at the caret, or fall back to the clipboard.

Injection is synthetic typing rather than a paste: it needs no clipboard access, does not
clobber what the user had copied, and works in the widest set of apps. Unicode is sent
directly (`KEYEVENTF_UNICODE` on Windows, `wtype`/`xdotool type` on Linux, System Events
`keystroke` on macOS), so no keyboard-layout translation is involved.

Synthetic input can be refused (an elevated window on Windows, a secure input field on
macOS, a Wayland compositor without a virtual-keyboard protocol), so any failure falls
back to the clipboard and the user is told.
"""

from __future__ import annotations

import ctypes
import shutil
import subprocess
import sys
import time
from typing import Callable, Optional

from .ports import InjectionResult

if sys.platform == "win32":  # ctypes.wintypes only exists on Windows
    import ctypes.wintypes  # noqa: F401


class FakeInjector:
    """Test double. `fail_with` makes it refuse, which is how step 10 is tested."""

    name = "fake"

    def __init__(self, fail_with: str = "", app: str = "TestApp", is_available: bool = True):
        self.fail_with = fail_with
        self.app = app
        self._available = is_available
        self.injected: list[str] = []

    def available(self) -> bool:
        return self._available

    def focused_app(self) -> str:
        return self.app

    def inject(self, text: str) -> InjectionResult:
        if self.fail_with:
            return InjectionResult(False, self.name, self.fail_with, self.app)
        self.injected.append(text)
        return InjectionResult(True, self.name, target_app=self.app)


class FakeClipboard:
    def __init__(self, works: bool = True):
        self.works = works
        self.contents: list[str] = []

    def copy(self, text: str) -> bool:
        if not self.works:
            return False
        self.contents.append(text)
        return True


class WindowsClipboard:
    name = "win32"

    def copy(self, text: str) -> bool:  # pragma: no cover - needs a Windows session
        CF_UNICODETEXT, GMEM_MOVEABLE = 13, 0x0002
        user32, kernel32 = ctypes.windll.user32, ctypes.windll.kernel32
        kernel32.GlobalAlloc.restype = ctypes.c_void_p
        kernel32.GlobalLock.restype = ctypes.c_void_p
        buffer = ctypes.create_unicode_buffer(text)
        size = ctypes.sizeof(buffer)
        if not user32.OpenClipboard(None):
            return False
        try:
            user32.EmptyClipboard()
            handle = kernel32.GlobalAlloc(GMEM_MOVEABLE, size)
            if not handle:
                return False
            locked = kernel32.GlobalLock(handle)
            ctypes.memmove(locked, ctypes.byref(buffer), size)
            kernel32.GlobalUnlock(handle)
            return bool(user32.SetClipboardData(CF_UNICODETEXT, ctypes.c_void_p(handle)))
        finally:
            user32.CloseClipboard()


class CommandClipboard:
    """pbcopy, wl-copy or xclip, whichever is installed."""

    name = "command"

    def __init__(self, argv: list[str]):
        self.argv = argv

    def copy(self, text: str) -> bool:  # pragma: no cover - needs a desktop session
        try:
            proc = subprocess.run(self.argv, input=text.encode("utf-8"), timeout=5)
            return proc.returncode == 0
        except (OSError, subprocess.TimeoutExpired):
            return False


def pick_clipboard():
    if sys.platform == "win32":
        return WindowsClipboard()
    if sys.platform == "darwin":
        return CommandClipboard(["pbcopy"])
    for argv in (["wl-copy"], ["xclip", "-selection", "clipboard"], ["xsel", "--clipboard", "--input"]):
        if shutil.which(argv[0]):
            return CommandClipboard(argv)
    return None


class WindowsInjector:
    """`SendInput` with `KEYEVENTF_UNICODE`: types text into the focused control."""

    name = "win32-sendinput"
    KEYEVENTF_UNICODE, KEYEVENTF_KEYUP, INPUT_KEYBOARD = 0x0004, 0x0002, 1

    def __init__(self, chunk: int = 200, pause_s: float = 0.001):
        self.chunk = chunk
        self.pause_s = pause_s

    def available(self) -> bool:
        return sys.platform == "win32"

    def focused_app(self) -> str:  # pragma: no cover - needs a Windows session
        try:
            user32 = ctypes.windll.user32
            hwnd = user32.GetForegroundWindow()
            length = user32.GetWindowTextLengthW(hwnd)
            buf = ctypes.create_unicode_buffer(length + 1)
            user32.GetWindowTextW(hwnd, buf, length + 1)
            return buf.value
        except Exception:
            return ""

    def inject(self, text: str) -> InjectionResult:  # pragma: no cover - needs a Windows session
        if not text:
            return InjectionResult(True, self.name)
        app = self.focused_app()
        try:
            sent = self._send(text)
        except OSError as exc:
            return InjectionResult(False, self.name, str(exc), app)
        if sent < len(_expand(text)):
            return InjectionResult(
                False, self.name,
                "SendInput was blocked (an elevated or protected window has focus)", app,
            )
        return InjectionResult(True, self.name, target_app=app)

    def _send(self, text: str) -> int:  # pragma: no cover - needs a Windows session
        wintypes = ctypes.wintypes
        ULONG_PTR = ctypes.c_ulonglong if ctypes.sizeof(ctypes.c_void_p) == 8 else ctypes.c_ulong

        class KEYBDINPUT(ctypes.Structure):
            _fields_ = [("wVk", wintypes.WORD), ("wScan", wintypes.WORD),
                        ("dwFlags", wintypes.DWORD), ("time", wintypes.DWORD),
                        ("dwExtraInfo", ULONG_PTR)]

        class INPUT(ctypes.Structure):
            class _U(ctypes.Union):
                _fields_ = [("ki", KEYBDINPUT)]

            _anonymous_ = ("u",)
            _fields_ = [("type", wintypes.DWORD), ("u", _U)]

        def key(code: int, up: bool) -> INPUT:
            flags = self.KEYEVENTF_UNICODE | (self.KEYEVENTF_KEYUP if up else 0)
            item = INPUT(type=self.INPUT_KEYBOARD)
            item.ki = KEYBDINPUT(0, code, flags, 0, 0)
            return item

        total = 0
        units = _expand(text)
        for start in range(0, len(units), self.chunk):
            batch = units[start : start + self.chunk]
            events = (INPUT * (len(batch) * 2))()
            for i, code in enumerate(batch):
                events[i * 2] = key(code, up=False)
                events[i * 2 + 1] = key(code, up=True)
            sent = ctypes.windll.user32.SendInput(len(events), ctypes.byref(events), ctypes.sizeof(INPUT))
            total += sent // 2
            if sent != len(events):
                break
            time.sleep(self.pause_s)
        return total


def _expand(text: str) -> list[int]:
    """UTF-16 code units. Characters outside the BMP are sent as two surrogates."""
    units: list[int] = []
    for ch in text.replace("\r\n", "\n"):
        if ch == "\n":
            units.append(0x000D)  # apps expect CR from a keyboard, not LF
            continue
        code = ord(ch)
        if code > 0xFFFF:
            code -= 0x10000
            units.append(0xD800 + (code >> 10))
            units.append(0xDC00 + (code & 0x3FF))
        else:
            units.append(code)
    return units


class MacInjector:
    """System Events keystroke. Needs the Accessibility permission."""

    name = "osascript-keystroke"

    def available(self) -> bool:
        return sys.platform == "darwin" and bool(shutil.which("osascript"))

    def focused_app(self) -> str:  # pragma: no cover - needs macOS
        script = 'tell application "System Events" to get name of first process whose frontmost is true'
        try:
            out = subprocess.run(["osascript", "-e", script], capture_output=True, text=True, timeout=5)
            return out.stdout.strip()
        except (OSError, subprocess.TimeoutExpired):
            return ""

    def inject(self, text: str) -> InjectionResult:  # pragma: no cover - needs macOS
        app = self.focused_app()
        escaped = text.replace("\\", "\\\\").replace('"', '\\"')
        script = f'tell application "System Events" to keystroke "{escaped}"'
        try:
            proc = subprocess.run(["osascript", "-e", script], capture_output=True, text=True, timeout=20)
        except (OSError, subprocess.TimeoutExpired) as exc:
            return InjectionResult(False, self.name, str(exc), app)
        if proc.returncode != 0:
            return InjectionResult(False, self.name, proc.stderr.strip()[:300], app)
        return InjectionResult(True, self.name, target_app=app)


class LinuxInjector:
    """`wtype` on Wayland, `xdotool type` on X11."""

    def __init__(self) -> None:
        if shutil.which("wtype"):
            self.name, self._argv = "wtype", ["wtype", "-"]
        elif shutil.which("xdotool"):
            self.name, self._argv = "xdotool", ["xdotool", "type", "--clearmodifiers", "--file", "-"]
        else:
            self.name, self._argv = "none", []

    def available(self) -> bool:
        return bool(self._argv)

    def focused_app(self) -> str:  # pragma: no cover - needs a desktop session
        if not shutil.which("xdotool"):
            return ""
        try:
            out = subprocess.run(["xdotool", "getactivewindow", "getwindowname"],
                                 capture_output=True, text=True, timeout=5)
            return out.stdout.strip()
        except (OSError, subprocess.TimeoutExpired):
            return ""

    def inject(self, text: str) -> InjectionResult:  # pragma: no cover - needs a desktop session
        if not self._argv:
            return InjectionResult(False, "none", "install wtype (Wayland) or xdotool (X11)")
        app = self.focused_app()
        try:
            proc = subprocess.run(self._argv, input=text.encode("utf-8"), capture_output=True, timeout=20)
        except (OSError, subprocess.TimeoutExpired) as exc:
            return InjectionResult(False, self.name, str(exc), app)
        if proc.returncode != 0:
            return InjectionResult(False, self.name, proc.stderr.decode(errors="replace")[:300], app)
        return InjectionResult(True, self.name, target_app=app)


class ClipboardFallbackInjector:
    """Wraps an injector so that a refusal turns into a clipboard copy and a notice."""

    name = "fallback"

    def __init__(self, primary, clipboard=None, notify: Optional[Callable[[str], None]] = None):
        self.primary = primary
        self.clipboard = clipboard
        self.notify = notify
        self.fallbacks = 0

    def available(self) -> bool:
        return bool(getattr(self.primary, "available", lambda: False)()) or self.clipboard is not None

    def focused_app(self) -> str:
        return self.primary.focused_app() if self.primary else ""

    def inject(self, text: str) -> InjectionResult:
        result = InjectionResult(False, "none", "no injector configured")
        if self.primary is not None and self.primary.available():
            result = self.primary.inject(text)
            if result.ok:
                return result
        if self.clipboard is None:
            self._say(f"Could not type into the focused app ({result.error}) and no clipboard is available.")
            return result
        if self.clipboard.copy(text):
            self.fallbacks += 1
            self._say(f"Could not type into {result.target_app or 'the focused app'} — copied to clipboard, press Ctrl+V.")
            return InjectionResult(True, "clipboard", result.error, result.target_app)
        self._say("Text injection and the clipboard both failed; the transcript is in your history.")
        return InjectionResult(False, "clipboard", result.error or "clipboard copy failed", result.target_app)

    def _say(self, message: str) -> None:
        if self.notify:
            self.notify(message)


def pick_injector():
    if sys.platform == "win32":
        return WindowsInjector()
    if sys.platform == "darwin":
        return MacInjector()
    return LinuxInjector()


def build_injector(config, notify: Optional[Callable[[str], None]] = None):
    primary = pick_injector()
    if isinstance(primary, WindowsInjector):
        primary.chunk = config.inject_chunk
    clipboard = pick_clipboard() if config.clipboard_fallback else None
    return ClipboardFallbackInjector(primary, clipboard, notify)
