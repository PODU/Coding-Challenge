# ccvoice — private, offline voice dictation

[Coding Challenge #135](https://codingchallenges.substack.com/p/coding-challenge-135-voice-dictation).

Hold a key, talk, let go: the words appear at the cursor of whatever app you were already
in — cleaned up, punctuated, and never having left the machine.

```
$ ccvoice run
Hold right ctrl to dictate. Ctrl+C to quit.

  [* REC 0:04] whisper-base.en  um so here is the plan first ship the fix second...

  ...and Notepad receives:

    So here is the plan:
    1. Ship the fix
    2. Tell the team

    I will send Claude Code the notes.
```

Python 3.10+, **no required dependencies**. Every OS and hardware touchpoint —
microphone, global hotkey, speech model, keystroke injection, permission state — sits
behind a port with a real adapter and a test fake, so the entire suite runs in a couple of
seconds with no microphone, no model weights, and no network.

## Quick start

```bash
pip install -e .[audio]                 # optional: sounddevice for capture
ccvoice setup                           # permission gate (mic + input injection)
ccvoice models download whisper-base.en # ~148 MB, once
ccvoice run
```

whisper.cpp does the actual recognising — put its `whisper-cli` binary on `PATH`
(`brew install whisper-cpp`, or build it from source), or `pip install .[fast]` to use
faster-whisper in-process instead.

Nothing to install yet? Everything but the microphone works standalone:

```bash
ccvoice format "um so first check the logs second restart it period"
ccvoice format --explain "he said open quote ship it close quote"
ccvoice commands            # every spoken formatting command
ccvoice doctor              # what this machine can and cannot do
```

## The challenge, step by step

| Step | What it asks for | Where it lives |
|---|---|---|
| 1 | Global hotkey, record while held | `hotkey.py` — `WH_KEYBOARD_LL` hook on Windows, pynput on mac/Linux, stdin toggle everywhere |
| 2 | Visual recording indicator | `indicator.py` — always-on-top tkinter pill, or a self-erasing console line |
| 3 | Local speech-to-text | `stt.py` + `netguard.py` — whisper.cpp / faster-whisper / vosk, with connections blocked while dictating |
| 4 | Multiple models, downloaded on demand | `models.py` — 4-model catalogue, resumable download with progress, instant switching |
| 5 | Live transcription preview | `stt.StreamingTranscriber` — re-transcribes a growing snapshot every 0.7s |
| 6 | Transcript cleanup | `cleanup.py` — fillers, stutters, punctuation, sentence casing |
| 7 | Spoken formatting commands | `commands.py` — "new line", "period", "open quote", and `literally` to escape them |
| 8 | List detection | `listfmt.py` — ordinal runs and "bullet point" become real lists |
| 9 | Direct text insertion | `inject.py` — `SendInput` / System Events / `wtype` / `xdotool` |
| 10 | Clipboard fallback | `inject.ClipboardFallbackInjector` — copies and tells you when typing is refused |
| 11 | First-launch setup | `permissions.SetupFlow` — will not advance past a denied permission |
| 12 | Revocation detection | `permissions.PermissionMonitor` — suspends dictation, resumes when restored |
| 13 | Custom dictionary | `config.Config.dictionary` + `cleanup.apply_dictionary` |
| 14 | Searchable history | `history.py` — SQLite, raw *and* cleaned text, date/model/app filters |
| 15 | Statistics | `history.Stats` — sessions, words, WPM, busiest day, per-model breakdown |

## Speaking to it

```
new line / new paragraph / tab       line and paragraph breaks
period, comma, question mark, ...    punctuation, attached correctly
open quote ... close quote           "quoted text"
open paren ... close paren           (parenthetical)
dash, slash                          joined with no spaces
literally <command>                  the words, not the command
first ... second ... third ...       a numbered list
bullet point ... bullet point ...    a bulleted list
```

Cleanup drops `um`, `uh`, `erm`, `you know`, `i mean`, collapses `the the`, capitalises
sentences and `I`, and adds the full stop you did not say. All of it is rule-based and
local — `ccvoice format --explain` shows exactly what each stage did, which matters when
you are deciding whether to trust the thing with your writing.

## Privacy

- The transcription path opens no sockets. `NetworkGuard` patches `socket.socket` for the
  duration of every dictation, so an accidental HTTP call raises instead of leaking
  speech. `ccvoice run --paranoid` keeps it armed for the whole session.
- `models.py` is the only module allowed to reach the network, and only when you ask for
  a model that is not on disk.
- Audio is never written to disk (`keep_audio` is off) — transcripts go to
  `~/.ccvoice/history.db`, which is yours, local, and `ccvoice history clear`-able.
- `test_offline.py` asserts a full dictate-and-inject cycle with the guard armed, so the
  claim is a test rather than a promise.

## Commands

```
ccvoice run [--model ID] [--hotkey KEY] [--paranoid] [--no-preview] [--no-inject]
ccvoice setup | doctor | commands
ccvoice models list | download ID | use ID | remove ID
ccvoice transcribe FILE.wav [--raw]
ccvoice format TEXT [--explain]
ccvoice dict list | add SPOKEN WRITTEN | rm SPOKEN
ccvoice history search [QUERY] [--limit N] [--since 7d] [--model ID] [--app NAME]
ccvoice history show ID | export FILE.jsonl | clear
ccvoice stats [--since 30d]
```

Configuration lives in `~/.ccvoice/config.json` (override the whole directory with
`CCVOICE_HOME`): hotkeys, model, preview interval, filler list, list style, dictionary.

## Platform notes

| | Hotkey | Injection | Notes |
|---|---|---|---|
| Windows | `WH_KEYBOARD_LL` hook (built in) | `SendInput` unicode | Fn is firmware-level and invisible; use right ctrl. Elevated windows refuse synthetic input (UIPI) — the clipboard fallback covers them |
| macOS | pynput / CGEventTap | System Events `keystroke` | needs Accessibility *and* Microphone permission; both are checked by `ccvoice setup` |
| Linux | pynput (X11) | `wtype` (Wayland) or `xdotool` (X11) | some Wayland compositors block virtual keyboards entirely — the fallback handles it |

Capture uses `sounddevice` when installed, otherwise pipes raw PCM out of
ffmpeg/arecord. `ccvoice doctor` prints what it found.

## Tests

```bash
pip install -e .[dev]
pytest -q          # 162 tests, all hermetic
```

One test file per challenge step, plus `test_e2e.py`, which drives a full press → speak →
release → cursor cycle through the fakes with the offline guard armed.
