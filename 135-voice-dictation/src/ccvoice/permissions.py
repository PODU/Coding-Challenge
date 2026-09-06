"""Permission checks at first launch, and noticing when they are revoked later.

The app needs the microphone and the ability to send input to other applications. Both
can be revoked in OS settings at any time and then fail confusingly (a silent recording,
or text that goes nowhere), so they are checked up front and watched afterwards.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import threading
import time
from typing import Callable, Iterable, Optional

from .ports import Permission, PermissionState

REQUIRED = (Permission.MICROPHONE, Permission.INPUT_INJECTION)


class FakePermissionChecker:
    """Test double: permissions are whatever the test says they are, and can change."""

    def __init__(self, granted: Optional[dict[Permission, bool]] = None):
        self.granted = dict(granted or {p: True for p in REQUIRED})
        self.requests: list[Permission] = []
        self.checks: list[Permission] = []

    def check(self, permission: Permission) -> PermissionState:
        self.checks.append(permission)
        ok = self.granted.get(permission, False)
        return PermissionState(permission, ok, detail="fake", remedy="flip it in the test")

    def request(self, permission: Permission) -> None:
        self.requests.append(permission)

    def grant(self, permission: Permission) -> None:
        self.granted[permission] = True

    def revoke(self, permission: Permission) -> None:
        self.granted[permission] = False


class WindowsPermissionChecker:
    """Microphone consent lives in the registry; synthetic input is allowed by default."""

    MIC_KEY = r"SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\microphone"

    def check(self, permission: Permission) -> PermissionState:
        if permission is Permission.MICROPHONE:
            return self._microphone()
        return PermissionState(
            permission,
            True,
            detail="SendInput is permitted for normal windows",
            remedy="Windows blocks synthetic input into elevated windows (UIPI). "
            "Run ccvoice as administrator only if you need to dictate into one.",
        )

    def _microphone(self) -> PermissionState:
        import winreg

        for root in (winreg.HKEY_CURRENT_USER, winreg.HKEY_LOCAL_MACHINE):
            for sub in (self.MIC_KEY, self.MIC_KEY + r"\NonPackaged"):
                try:
                    with winreg.OpenKey(root, sub) as key:
                        value, _ = winreg.QueryValueEx(key, "Value")
                except OSError:
                    continue
                if str(value).lower() == "deny":
                    return PermissionState(
                        Permission.MICROPHONE, False,
                        detail="microphone access is set to Deny",
                        remedy="Settings > Privacy & security > Microphone: turn on "
                        "'Microphone access' and 'Let desktop apps access your microphone'",
                    )
        return PermissionState(Permission.MICROPHONE, True, detail="microphone access allowed")

    def request(self, permission: Permission) -> None:  # pragma: no cover - opens a GUI
        pane = "privacy-microphone" if permission is Permission.MICROPHONE else "privacy"
        subprocess.run(["cmd", "/c", "start", "", f"ms-settings:{pane}"], check=False)


class MacPermissionChecker:
    """TCC cannot be queried directly, so probe the APIs and read the failure."""

    def check(self, permission: Permission) -> PermissionState:
        if permission is Permission.INPUT_INJECTION:
            return self._accessibility()
        return self._microphone()

    def _accessibility(self) -> PermissionState:  # pragma: no cover - needs macOS
        script = 'tell application "System Events" to get name of first process whose frontmost is true'
        try:
            proc = subprocess.run(["osascript", "-e", script], capture_output=True, text=True, timeout=10)
        except (OSError, subprocess.TimeoutExpired) as exc:
            return PermissionState(Permission.INPUT_INJECTION, False, str(exc), self._remedy_accessibility())
        if proc.returncode == 0:
            return PermissionState(Permission.INPUT_INJECTION, True, "System Events responded")
        return PermissionState(
            Permission.INPUT_INJECTION, False,
            proc.stderr.strip()[:200], self._remedy_accessibility(),
        )

    def _microphone(self) -> PermissionState:  # pragma: no cover - needs macOS
        if not shutil.which("ffmpeg"):
            return PermissionState(Permission.MICROPHONE, True, "cannot probe without ffmpeg", unknown=True)
        try:
            proc = subprocess.run(
                ["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "avfoundation",
                 "-i", ":0", "-t", "0.2", "-f", "null", "-"],
                capture_output=True, text=True, timeout=15,
            )
        except (OSError, subprocess.TimeoutExpired) as exc:
            return PermissionState(Permission.MICROPHONE, False, str(exc), self._remedy_mic())
        if proc.returncode == 0:
            return PermissionState(Permission.MICROPHONE, True, "captured a test frame")
        return PermissionState(Permission.MICROPHONE, False, proc.stderr.strip()[:200], self._remedy_mic())

    @staticmethod
    def _remedy_accessibility() -> str:
        return ("System Settings > Privacy & Security > Accessibility: add and enable your "
                "terminal (or the ccvoice app), then restart it")

    @staticmethod
    def _remedy_mic() -> str:
        return "System Settings > Privacy & Security > Microphone: enable your terminal"

    def request(self, permission: Permission) -> None:  # pragma: no cover - opens a GUI
        pane = "Privacy_Microphone" if permission is Permission.MICROPHONE else "Privacy_Accessibility"
        subprocess.run(
            ["open", f"x-apple.systempreferences:com.apple.preference.security?{pane}"], check=False
        )


class LinuxPermissionChecker:
    """Linux has no consent dialogs; we check device access and an injection tool."""

    def check(self, permission: Permission) -> PermissionState:
        if permission is Permission.MICROPHONE:
            readable = os.path.isdir("/dev/snd") and os.access("/dev/snd", os.R_OK)
            if readable or shutil.which("pipewire") or shutil.which("pulseaudio"):
                return PermissionState(permission, True, "audio devices reachable")
            return PermissionState(
                permission, False, "/dev/snd is not readable",
                "add your user to the `audio` group (`sudo usermod -aG audio $USER`) and log back in",
            )
        wayland = os.environ.get("XDG_SESSION_TYPE", "").lower() == "wayland"
        tool = "wtype" if wayland else "xdotool"
        if shutil.which(tool):
            return PermissionState(permission, True, f"{tool} is installed")
        return PermissionState(
            permission, False, f"{tool} not found",
            f"install {tool} (`sudo apt install {tool}`)"
            + (" — some compositors also need the virtual-keyboard protocol enabled" if wayland else ""),
        )

    def request(self, permission: Permission) -> None:
        return None


def pick_checker():
    if sys.platform == "win32":
        return WindowsPermissionChecker()
    if sys.platform == "darwin":
        return MacPermissionChecker()
    return LinuxPermissionChecker()


class CachingChecker:
    """Wraps a checker so the background monitor does not hammer the OS APIs."""

    def __init__(self, inner, ttl_s: float = 5.0, clock: Callable[[], float] = time.monotonic):
        self.inner = inner
        self.ttl_s = ttl_s
        self._clock = clock
        self._cache: dict[Permission, tuple[float, PermissionState]] = {}

    def check(self, permission: Permission) -> PermissionState:
        now = self._clock()
        hit = self._cache.get(permission)
        if hit and now - hit[0] < self.ttl_s:
            return hit[1]
        state = self.inner.check(permission)
        self._cache[permission] = (now, state)
        return state

    def request(self, permission: Permission) -> None:
        self._cache.pop(permission, None)
        self.inner.request(permission)

    def invalidate(self) -> None:
        self._cache.clear()


class SetupFlow:
    """First-run setup. Won't finish while a permission is still denied."""

    def __init__(
        self,
        checker,
        prompt: Callable[[str], str] = input,
        out: Callable[[str], None] = print,
        max_rounds: int = 25,
    ):
        self.checker = checker
        self.prompt = prompt
        self.out = out
        self.max_rounds = max_rounds
        self.granted: list[Permission] = []

    def run(self, required: Iterable[Permission] = REQUIRED) -> bool:
        self.out("ccvoice first-run setup — two permissions are needed.\n")
        for permission in required:
            if not self._gate(permission):
                self.out(f"\nSetup stopped: {permission.value} is still not granted.")
                return False
            self.granted.append(permission)
        self.out("\nAll permissions granted. You are ready to dictate.")
        return True

    def _gate(self, permission: Permission) -> bool:
        for round_no in range(self.max_rounds):
            state = self.checker.check(permission)
            if state.granted:
                mark = "?" if state.unknown else "OK"
                self.out(f"[{mark}] {permission.value}: {state.detail}")
                return True
            self.out(f"[--] {permission.value}: {state.detail}")
            if state.remedy:
                self.out(f"     fix: {state.remedy}")
            if round_no == 0:
                self.checker.request(permission)
            if hasattr(self.checker, "invalidate"):
                self.checker.invalidate()
            answer = self.prompt("     press Enter once granted, or 'q' to quit: ").strip().lower()
            if answer in {"q", "quit", "exit"}:
                return False
        return False


class PermissionMonitor:
    """Polls for permissions being revoked after setup."""

    def __init__(
        self,
        checker,
        on_revoked: Callable[[PermissionState], None],
        permissions: Iterable[Permission] = REQUIRED,
        interval_s: float = 15.0,
        on_restored: Optional[Callable[[PermissionState], None]] = None,
    ):
        self.checker = checker
        self.on_revoked = on_revoked
        self.on_restored = on_restored
        self.permissions = tuple(permissions)
        self.interval_s = interval_s
        self.state: dict[Permission, bool] = {}
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None

    def poll(self) -> list[PermissionState]:
        """One sweep. Returns the permissions that flipped granted -> denied."""
        if hasattr(self.checker, "invalidate"):
            self.checker.invalidate()
        revoked: list[PermissionState] = []
        for permission in self.permissions:
            current = self.checker.check(permission)
            previous = self.state.get(permission)
            self.state[permission] = current.granted
            if previous is None:
                continue
            if previous and not current.granted:
                revoked.append(current)
                self.on_revoked(current)
            elif not previous and current.granted and self.on_restored:
                self.on_restored(current)
        return revoked

    def start(self) -> None:
        self.poll()  # establish the baseline before watching for changes
        self._stop.clear()
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()

    def _loop(self) -> None:
        while not self._stop.wait(self.interval_s):
            self.poll()

    def stop(self) -> None:
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=2)
            self._thread = None
