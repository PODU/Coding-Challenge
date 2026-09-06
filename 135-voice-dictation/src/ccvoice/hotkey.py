"""Global push-to-talk hotkeys.

Push-to-talk needs key *down* and key *up*, not "a key was typed", which rules out
`RegisterHotKey` on Windows and plain terminal input everywhere. The Windows adapter
therefore installs a `WH_KEYBOARD_LL` hook on its own thread with its own message loop;
the mac/Linux adapters use pynput (CGEventTap / X11 records) when it is installed.

`StdinHotkeyListener` is the always-available fallback: press Enter to start, Enter again
to stop. It is what makes the app demoable on a headless box or in a container.
"""

from __future__ import annotations

import ctypes
import sys
import threading
from typing import Callable, Optional

from .ports import HotkeyUnavailable

# Spoken/config name -> Windows virtual key code.
VK_CODES = {
    "right ctrl": 0xA3,
    "left ctrl": 0xA2,
    "right alt": 0xA5,
    "left alt": 0xA4,
    "right shift": 0xA1,
    "left shift": 0xA0,
    "caps lock": 0x14,
    "scroll lock": 0x91,
    "pause": 0x13,
    "right cmd": 0x5C,  # right Windows key
    "left cmd": 0x5B,
    "f13": 0x7C,
    "f14": 0x7D,
    "f15": 0x7E,
    "insert": 0x2D,
}

# Keys the OS never surfaces to userspace on some platforms.
UNSUPPORTED_HINTS = {
    "fn": "the Fn key is handled in firmware and is not visible to Windows or X11; "
    "use 'right ctrl', 'right alt' or 'caps lock' instead",
}


def normalise(combo: str) -> str:
    return " ".join(combo.lower().replace("_", " ").replace("-", " ").split())


class FakeHotkeyListener:
    """Test double: the test calls `press()` / `release()` itself."""

    def __init__(self) -> None:
        self.combo = ""
        self.started = False
        self._on_press: Optional[Callable[[], None]] = None
        self._on_release: Optional[Callable[[], None]] = None
        self.events: list[str] = []

    def register(self, combo: str, on_press: Callable[[], None], on_release: Callable[[], None]) -> None:
        self.combo = normalise(combo)
        self._on_press, self._on_release = on_press, on_release

    def start(self) -> None:
        self.started = True

    def stop(self) -> None:
        self.started = False

    def press(self) -> None:
        if not self.started or not self._on_press:
            return
        self.events.append("press")
        self._on_press()

    def release(self) -> None:
        if not self.started or not self._on_release:
            return
        self.events.append("release")
        self._on_release()

    def hold(self) -> None:
        self.press()
        self.release()


class StdinHotkeyListener:
    """Enter-to-toggle fallback. Always available, never global."""

    name = "stdin"

    def __init__(self, stream=None) -> None:
        self.stream = stream or sys.stdin
        self._on_press: Optional[Callable[[], None]] = None
        self._on_release: Optional[Callable[[], None]] = None
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None

    def register(self, combo: str, on_press, on_release) -> None:
        self._on_press, self._on_release = on_press, on_release

    def start(self) -> None:
        self._stop.clear()
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()

    def _loop(self) -> None:
        talking = False
        while not self._stop.is_set():
            line = self.stream.readline()
            if not line:
                break
            if line.strip().lower() in {"q", "quit", "exit"}:
                if talking and self._on_release:
                    self._on_release()
                self._stop.set()
                break
            talking = not talking
            cb = self._on_press if talking else self._on_release
            if cb:
                cb()

    def stop(self) -> None:
        self._stop.set()

    def wait(self, timeout: Optional[float] = None) -> None:
        if self._thread:
            self._thread.join(timeout)


class WindowsHotkeyListener:
    """`SetWindowsHookEx(WH_KEYBOARD_LL)` on a dedicated thread with a message pump."""

    name = "win32-ll-hook"
    WH_KEYBOARD_LL = 13
    WM_KEYDOWN, WM_KEYUP = 0x0100, 0x0101
    WM_SYSKEYDOWN, WM_SYSKEYUP = 0x0104, 0x0105

    def __init__(self) -> None:
        if sys.platform != "win32":  # pragma: no cover - platform guard
            raise HotkeyUnavailable("WindowsHotkeyListener requires Windows")
        self.vks: set[int] = set()
        self._down = False
        self._on_press: Optional[Callable[[], None]] = None
        self._on_release: Optional[Callable[[], None]] = None
        self._thread: Optional[threading.Thread] = None
        self._thread_id: Optional[int] = None
        self._hook = None

    def register(self, combo: str, on_press, on_release) -> None:
        for name in combo.split(","):
            key = normalise(name)
            if key in UNSUPPORTED_HINTS:
                raise HotkeyUnavailable(UNSUPPORTED_HINTS[key])
            if key not in VK_CODES:
                raise HotkeyUnavailable(f"unknown key {key!r}; known: {', '.join(sorted(VK_CODES))}")
            self.vks.add(VK_CODES[key])
        self._on_press, self._on_release = on_press, on_release

    def start(self) -> None:  # pragma: no cover - needs a Windows desktop session
        self._thread = threading.Thread(target=self._pump, daemon=True)
        self._thread.start()

    def _pump(self) -> None:  # pragma: no cover - needs a Windows desktop session
        user32 = ctypes.windll.user32
        kernel32 = ctypes.windll.kernel32
        self._thread_id = kernel32.GetCurrentThreadId()

        LRESULT = ctypes.c_ssize_t
        HOOKPROC = ctypes.WINFUNCTYPE(LRESULT, ctypes.c_int, ctypes.c_void_p, ctypes.c_void_p)

        def callback(code, wparam, lparam):
            if code >= 0:
                vk = ctypes.cast(lparam, ctypes.POINTER(ctypes.c_ulong))[0]
                if vk in self.vks:
                    if wparam in (self.WM_KEYDOWN, self.WM_SYSKEYDOWN) and not self._down:
                        self._down = True
                        self._fire(self._on_press)
                    elif wparam in (self.WM_KEYUP, self.WM_SYSKEYUP) and self._down:
                        self._down = False
                        self._fire(self._on_release)
            return user32.CallNextHookEx(None, code, wparam, lparam)

        self._proc = HOOKPROC(callback)  # keep a reference or it is collected mid-hook
        self._hook = user32.SetWindowsHookExW(self.WH_KEYBOARD_LL, self._proc, None, 0)
        if not self._hook:
            raise HotkeyUnavailable("SetWindowsHookExW failed")

        import ctypes.wintypes as wintypes

        msg = wintypes.MSG()
        while user32.GetMessageW(ctypes.byref(msg), None, 0, 0) > 0:
            user32.TranslateMessage(ctypes.byref(msg))
            user32.DispatchMessageW(ctypes.byref(msg))
        user32.UnhookWindowsHookEx(self._hook)

    @staticmethod
    def _fire(cb: Optional[Callable[[], None]]) -> None:  # pragma: no cover
        if cb:
            threading.Thread(target=cb, daemon=True).start()  # never block the hook chain

    def stop(self) -> None:  # pragma: no cover - needs a Windows desktop session
        if self._thread_id:
            ctypes.windll.user32.PostThreadMessageW(self._thread_id, 0x0012, 0, 0)  # WM_QUIT


class PynputHotkeyListener:
    """macOS (CGEventTap) and Linux (X11) adapter, via the optional `pynput` package."""

    name = "pynput"

    def __init__(self) -> None:
        try:
            from pynput import keyboard  # noqa: F401
        except Exception as exc:  # pragma: no cover - depends on the host
            raise HotkeyUnavailable(
                "global hotkeys on this platform need `pynput` (pip install pynput)"
            ) from exc
        self.keys: set[str] = set()
        self._down = False
        self._listener = None
        self._on_press = None
        self._on_release = None

    def register(self, combo: str, on_press, on_release) -> None:
        for name in combo.split(","):
            key = normalise(name)
            if key in UNSUPPORTED_HINTS and sys.platform != "darwin":
                raise HotkeyUnavailable(UNSUPPORTED_HINTS[key])
            self.keys.add(key.replace(" ", "_"))
        self._on_press, self._on_release = on_press, on_release

    def start(self) -> None:  # pragma: no cover - needs a desktop session
        from pynput import keyboard

        def matches(key) -> bool:
            name = getattr(key, "name", None) or getattr(key, "char", None) or ""
            return name.lower() in self.keys

        def down(key):
            if matches(key) and not self._down:
                self._down = True
                if self._on_press:
                    self._on_press()

        def up(key):
            if matches(key) and self._down:
                self._down = False
                if self._on_release:
                    self._on_release()

        self._listener = keyboard.Listener(on_press=down, on_release=up)
        self._listener.start()

    def stop(self) -> None:  # pragma: no cover - needs a desktop session
        if self._listener:
            self._listener.stop()


def pick_listener(prefer: str = "auto"):
    """Best global-hotkey backend for this host, falling back to stdin toggling."""
    if prefer == "stdin":
        return StdinHotkeyListener()
    errors = []
    for factory in ((WindowsHotkeyListener,) if sys.platform == "win32" else (PynputHotkeyListener,)):
        try:
            return factory()
        except HotkeyUnavailable as exc:
            errors.append(str(exc))
    if prefer == "global":
        raise HotkeyUnavailable("; ".join(errors) or "no global hotkey backend")
    return StdinHotkeyListener()
