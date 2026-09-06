"""The recording indicator and live transcription preview.

There is a borderless always-on-top Tk overlay (tkinter ships with Python) and an ANSI
console line for headless use. Both show that we are listening, for how long, and the
partial transcript so far.
"""

from __future__ import annotations

import queue
import sys
import threading
import time
from typing import Optional


class SpyIndicator:
    """Test double: records the lifecycle so tests can assert it exactly."""

    def __init__(self) -> None:
        self.calls: list[tuple[str, str]] = []
        self.previews: list[str] = []
        self.notifications: list[str] = []
        self.visible = False

    def show(self, model_id: str) -> None:
        self.visible = True
        self.calls.append(("show", model_id))

    def preview(self, text: str) -> None:
        self.previews.append(text)
        self.calls.append(("preview", text))

    def hide(self) -> None:
        self.visible = False
        self.calls.append(("hide", ""))

    def notify(self, message: str) -> None:
        self.notifications.append(message)
        self.calls.append(("notify", message))


class NullIndicator:
    def show(self, model_id: str) -> None: ...

    def preview(self, text: str) -> None: ...

    def hide(self) -> None: ...

    def notify(self, message: str) -> None: ...


class ConsoleIndicator:
    """One self-erasing terminal line: `[* REC 0:04] whisper-base.en  partial text...`"""

    name = "console"

    def __init__(self, stream=None, width: int = 100, clock=time.monotonic) -> None:
        self.stream = stream or sys.stderr
        self.width = width
        self._clock = clock
        self._started = 0.0
        self._model = ""
        self._text = ""
        self._active = False

    def show(self, model_id: str) -> None:
        self._model, self._text, self._started, self._active = model_id, "", self._clock(), True
        self._draw()

    def preview(self, text: str) -> None:
        self._text = text
        if self._active:
            self._draw()

    def hide(self) -> None:
        if self._active:
            self._active = False
            self._erase()

    def notify(self, message: str) -> None:
        self._erase()
        self.stream.write(f"{message}\n")
        self.stream.flush()

    def _draw(self) -> None:
        elapsed = self._clock() - self._started
        head = f"\r[* REC {int(elapsed // 60)}:{int(elapsed % 60):02d}] {self._model}  "
        body = self._text.replace("\n", " / ")
        room = max(0, self.width - len(head))
        if len(body) > room:
            body = "..." + body[-(room - 3) :] if room > 3 else ""
        self.stream.write(head + body.ljust(room))
        self.stream.flush()

    def _erase(self) -> None:
        self.stream.write("\r" + " " * self.width + "\r")
        self.stream.flush()


class TkOverlayIndicator:
    """Borderless, always-on-top floating pill near the bottom of the primary screen.

    Tk insists on owning its thread, so the widget lives in a worker thread and the app
    talks to it through a queue; every UI mutation happens inside the Tk event loop.
    """

    name = "overlay"

    def __init__(self, anchor: str = "bottom", auto_hide_s: float = 4.0) -> None:
        try:
            import tkinter  # noqa: F401
        except Exception as exc:  # pragma: no cover - depends on the host
            raise RuntimeError(f"tkinter unavailable: {exc}") from exc
        self.anchor = anchor
        self.auto_hide_s = auto_hide_s
        self._q: "queue.Queue[tuple[str, str]]" = queue.Queue()
        self._thread: Optional[threading.Thread] = None
        self._ready = threading.Event()

    def show(self, model_id: str) -> None:
        self._ensure()
        self._q.put(("show", model_id))

    def preview(self, text: str) -> None:
        self._q.put(("preview", text))

    def hide(self) -> None:
        self._q.put(("hide", ""))

    def notify(self, message: str) -> None:
        self._ensure()
        self._q.put(("notify", message))

    def close(self) -> None:
        self._q.put(("quit", ""))
        if self._thread:
            self._thread.join(timeout=2)

    def _ensure(self) -> None:  # pragma: no cover - needs a display
        if self._thread and self._thread.is_alive():
            return
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()
        self._ready.wait(timeout=5)

    def _run(self) -> None:  # pragma: no cover - needs a display
        import tkinter as tk

        root = tk.Tk()
        root.withdraw()
        root.overrideredirect(True)
        root.attributes("-topmost", True)
        try:
            root.attributes("-alpha", 0.92)
        except tk.TclError:
            pass
        frame = tk.Frame(root, bg="#141414", padx=14, pady=8)
        frame.pack(fill="both", expand=True)
        dot = tk.Label(frame, text="●", fg="#ff4b4b", bg="#141414", font=("Segoe UI", 14))
        dot.pack(side="left")
        label = tk.Label(
            frame, text="", fg="#f2f2f2", bg="#141414", font=("Segoe UI", 11),
            wraplength=560, justify="left",
        )
        label.pack(side="left", padx=(10, 0))

        state = {"on": False, "blink": True, "started": 0.0, "hide_at": 0.0}

        def place() -> None:
            root.update_idletasks()
            w, h = root.winfo_reqwidth(), root.winfo_reqheight()
            sw, sh = root.winfo_screenwidth(), root.winfo_screenheight()
            y = sh - h - 90 if self.anchor == "bottom" else 60
            root.geometry(f"+{(sw - w) // 2}+{y}")

        def tick() -> None:
            try:
                while True:
                    kind, payload = self._q.get_nowait()
                    if kind == "quit":
                        root.destroy()
                        return
                    if kind == "show":
                        state.update(on=True, started=time.monotonic(), hide_at=0.0)
                        label.config(text=f"Listening — {payload}")
                        root.deiconify()
                        place()
                    elif kind == "preview" and state["on"]:
                        elapsed = time.monotonic() - state["started"]
                        label.config(text=f"{payload or 'Listening...'}  ({elapsed:0.1f}s)")
                        place()
                    elif kind == "hide":
                        state["on"] = False
                        root.withdraw()
                    elif kind == "notify":
                        state.update(on=False, hide_at=time.monotonic() + self.auto_hide_s)
                        label.config(text=payload)
                        dot.config(fg="#4bd07a")
                        root.deiconify()
                        place()
            except queue.Empty:
                pass
            if state["hide_at"] and time.monotonic() > state["hide_at"]:
                state["hide_at"] = 0.0
                dot.config(fg="#ff4b4b")
                root.withdraw()
            if state["on"]:
                state["blink"] = not state["blink"]
                dot.config(fg="#ff4b4b" if state["blink"] else "#5a1c1c")
            root.after(120, tick)

        self._ready.set()
        root.after(0, tick)
        root.mainloop()


def pick_indicator(kind: str = "auto"):
    """Overlay when there is a display, console otherwise."""
    if kind == "console":
        return ConsoleIndicator()
    if kind == "none":
        return NullIndicator()
    try:
        return TkOverlayIndicator()
    except Exception:
        return ConsoleIndicator()
