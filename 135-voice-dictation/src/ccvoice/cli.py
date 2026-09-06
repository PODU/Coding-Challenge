"""The `ccvoice` command line."""

from __future__ import annotations

import argparse
import logging
import sys
import time
from pathlib import Path
from typing import Optional

from . import commands as spoken
from .app import DictationApp
from .audio import pick_recorder
from .config import Config, default_home
from .history import HistoryStore, format_rows, parse_since
from .indicator import pick_indicator
from .inject import build_injector
from .models import DownloadProgress, ModelError, ModelManager
from .netguard import NetworkGuard
from .permissions import (
    REQUIRED,
    CachingChecker,
    PermissionMonitor,
    SetupFlow,
    pick_checker,
)
from .pipeline import TextPipeline
from .ports import AudioClip, AudioUnavailable, TranscriptionError
from .stt import build_engine


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="ccvoice", description="Private, offline voice dictation.")
    parser.add_argument("--home", type=Path, default=None, help="app data directory (default ~/.ccvoice)")
    parser.add_argument("-v", "--verbose", action="store_true", help="log every step")
    sub = parser.add_subparsers(dest="command", required=True)

    run = sub.add_parser("run", help="start the dictation daemon")
    run.add_argument("--model", default="", help="model id to dictate with")
    run.add_argument("--hotkey", default="", help="override the push-to-talk key")
    run.add_argument("--hotkey-backend", choices=("auto", "global", "stdin"), default="auto")
    run.add_argument("--indicator", choices=("auto", "overlay", "console", "none"), default="auto")
    run.add_argument("--paranoid", action="store_true", help="keep the offline guard armed all session")
    run.add_argument("--no-preview", action="store_true", help="disable live transcription preview")
    run.add_argument("--no-inject", action="store_true", help="print the text instead of typing it")

    sub.add_parser("setup", help="first-launch permission flow")
    sub.add_parser("doctor", help="report adapters, permissions and models")
    sub.add_parser("commands", help="list the spoken formatting commands")

    models = sub.add_parser("models", help="manage speech models").add_subparsers(dest="action", required=True)
    models.add_parser("list", help="show the catalogue")
    dl = models.add_parser("download", help="fetch a model")
    dl.add_argument("model_id")
    use = models.add_parser("use", help="switch the active model")
    use.add_argument("model_id")
    rm = models.add_parser("remove", help="delete downloaded weights")
    rm.add_argument("model_id")

    transcribe = sub.add_parser("transcribe", help="run a wav file through the whole pipeline")
    transcribe.add_argument("path", type=Path)
    transcribe.add_argument("--model", default="")
    transcribe.add_argument("--raw", action="store_true", help="show the untouched engine output too")

    fmt = sub.add_parser("format", help="run text through the cleanup pipeline")
    fmt.add_argument("text", nargs="+")
    fmt.add_argument("--explain", action="store_true", help="show what each stage did")

    dictionary = sub.add_parser("dict", help="custom word corrections").add_subparsers(
        dest="action", required=True
    )
    dictionary.add_parser("list")
    add = dictionary.add_parser("add")
    add.add_argument("spoken")
    add.add_argument("written", nargs="+")
    remove = dictionary.add_parser("rm")
    remove.add_argument("spoken")

    history = sub.add_parser("history", help="browse past dictations").add_subparsers(
        dest="action", required=True
    )
    search = history.add_parser("search")
    search.add_argument("query", nargs="*", default=[])
    search.add_argument("--limit", type=int, default=20)
    search.add_argument("--since", default="", help="7d, 12h or an ISO date")
    search.add_argument("--model", default="")
    search.add_argument("--app", default="")
    show = history.add_parser("show")
    show.add_argument("id", type=int)
    export = history.add_parser("export")
    export.add_argument("path", type=Path)
    history.add_parser("clear")

    stats = sub.add_parser("stats", help="aggregate dictation statistics")
    stats.add_argument("--since", default="")

    return parser


def main(argv: Optional[list[str]] = None) -> int:
    args = build_parser().parse_args(argv)
    logging.basicConfig(
        level=logging.INFO if args.verbose else logging.WARNING,
        format="%(asctime)s %(levelname)-7s %(message)s",
        datefmt="%H:%M:%S",
    )
    home = args.home or default_home()
    config = Config.load(home)

    handlers = {
        "run": cmd_run,
        "setup": cmd_setup,
        "doctor": cmd_doctor,
        "commands": cmd_commands,
        "models": cmd_models,
        "transcribe": cmd_transcribe,
        "format": cmd_format,
        "dict": cmd_dict,
        "history": cmd_history,
        "stats": cmd_stats,
    }
    try:
        return handlers[args.command](args, config, home)
    except (ModelError, TranscriptionError, AudioUnavailable) as exc:
        print(f"ccvoice: {exc}", file=sys.stderr)
        return 2
    except KeyboardInterrupt:
        return 130


def cmd_run(args, config: Config, home: Path) -> int:
    if args.model:
        config.model_id = args.model
    if args.hotkey:
        config.hotkey = args.hotkey
    if args.no_preview:
        config.live_preview = False
    if args.paranoid:
        config.paranoid = True

    checker = CachingChecker(pick_checker())
    if not config.setup_complete:
        print("Running first-launch setup (ccvoice setup re-runs it).\n")
        if not SetupFlow(checker).run():
            return 3
        config.setup_complete = True
        config.save(home)

    manager = ModelManager(home)
    if not manager.is_installed(config.model_id):
        print(f"Model {config.model_id} is not downloaded yet.")
        _download(manager, config.model_id)
    engine = build_engine(config, manager)

    from .hotkey import pick_listener

    indicator = pick_indicator(args.indicator)
    injector = _PrintInjector() if args.no_inject else build_injector(config, indicator.notify)
    guard = NetworkGuard()
    app = DictationApp(
        config=config,
        recorder=pick_recorder(),
        engine=engine,
        injector=injector,
        indicator=indicator,
        history=HistoryStore(home / "history.db"),
        hotkey=pick_listener(args.hotkey_backend),
        guard=guard,
    )

    monitor = PermissionMonitor(
        checker, app.on_permission_revoked, REQUIRED,
        interval_s=config.permission_poll_s, on_restored=app.on_permission_restored,
    )

    if config.paranoid:
        guard.arm()
    app.start()
    monitor.start()
    backend = getattr(app.hotkey, "name", "hotkey")
    if backend == "stdin":
        print("Press Enter to start dictating, Enter again to stop, 'q' to quit.")
    else:
        print(f"Hold {config.hotkey} to dictate. Ctrl+C to quit.")
    try:
        while True:
            time.sleep(0.25)
    except KeyboardInterrupt:
        print("\nstopping...")
    finally:
        monitor.stop()
        app.stop()
        if config.paranoid:
            guard.disarm()
    return 0


def cmd_setup(_args, config: Config, home: Path) -> int:
    flow = SetupFlow(CachingChecker(pick_checker()))
    ok = flow.run()
    config.setup_complete = ok
    config.save(home)
    return 0 if ok else 3


def cmd_doctor(_args, config: Config, home: Path) -> int:
    from .audio import capture_argv
    from .hotkey import pick_listener
    from .inject import pick_clipboard, pick_injector
    from .stt import WhisperCppEngine

    print(f"home            {home}")
    print(f"config          {Config.path(home)}")
    print(f"active model    {config.model_id}")
    print(f"hotkeys         {', '.join(config.hotkeys())}")

    try:
        recorder = pick_recorder()
        print(f"audio capture   OK ({getattr(recorder, 'name', type(recorder).__name__)})")
    except AudioUnavailable as exc:
        print(f"audio capture   MISSING — {exc}")
    argv = capture_argv()
    if argv:
        print(f"                fallback: {' '.join(argv[:5])} ...")

    listener = pick_listener()
    print(f"hotkey backend  {getattr(listener, 'name', type(listener).__name__)}")

    injector = pick_injector()
    print(f"text injection  {injector.name} ({'available' if injector.available() else 'unavailable'})")
    clipboard = pick_clipboard()
    print(f"clipboard       {getattr(clipboard, 'name', 'none')}")
    print(f"whisper binary  {WhisperCppEngine.find_binary() or 'not found on PATH'}")

    manager = ModelManager(home)
    installed = manager.installed()
    print(f"models          {', '.join(installed) if installed else 'none downloaded'}")

    checker = pick_checker()
    for permission in REQUIRED:
        state = checker.check(permission)
        mark = "unknown" if state.unknown else ("granted" if state.granted else "DENIED")
        print(f"perm {permission.value:<16} {mark} — {state.detail}")
        if not state.granted and state.remedy:
            print(f"     fix: {state.remedy}")

    store = HistoryStore(home / "history.db")
    print(f"history         {store.count()} dictations at {store.path}")
    store.close()
    return 0


def cmd_commands(_args, _config: Config, _home: Path) -> int:
    print("Say these while dictating:\n")
    for phrase, inserts in spoken.describe():
        print(f"  {phrase:<22} -> {inserts}")
    print("\n  literally <phrase>     -> the words themselves, not the command")
    print("  first ... second ...   -> a numbered list")
    print("  bullet point ...       -> a bulleted list")
    return 0


def cmd_models(args, config: Config, home: Path) -> int:
    manager = ModelManager(home)
    if args.action == "list":
        for model_id, spec, installed in manager.describe():
            active = " *active*" if model_id == config.model_id else ""
            mark = "downloaded" if installed else "not downloaded"
            print(f"{model_id:<18} {spec.engine:<14} {spec.size_mb:6.0f} MB  {mark}{active}")
            print(f"{'':<18} speed: {spec.speed}")
            print(f"{'':<18} accuracy: {spec.accuracy}")
        return 0
    if args.action == "download":
        _download(manager, args.model_id)
        return 0
    if args.action == "use":
        ModelManager.spec(args.model_id)  # validates the id
        if not manager.is_installed(args.model_id):
            _download(manager, args.model_id)
        config.model_id = args.model_id
        config.save(home)
        print(f"active model is now {args.model_id}")
        return 0
    if args.action == "remove":
        print("removed" if manager.remove(args.model_id) else "nothing to remove")
        return 0
    return 1


def _download(manager: ModelManager, model_id: str) -> None:
    spec = ModelManager.spec(model_id)
    print(f"downloading {model_id} ({spec.size_mb:.0f} MB) from {spec.url}")
    last = [0.0]

    def progress(update: DownloadProgress) -> None:
        now = time.monotonic()
        if update.done or now - last[0] > 0.1:
            last[0] = now
            end = "\n" if update.done else ""
            print("\r" + update.bar(), end=end, flush=True)

    manager.ensure(model_id, progress)
    print(f"ready: {manager.path(model_id)}")


def cmd_transcribe(args, config: Config, home: Path) -> int:
    clip = AudioClip.from_wav(args.path)
    manager = ModelManager(home)
    engine = build_engine(config, manager, args.model)
    guard = NetworkGuard()
    with guard.offline():
        transcript = engine.transcribe(clip)
    text = TextPipeline(config).run(transcript.text)
    if args.raw:
        print(f"raw   : {transcript.text}")
        print(f"clean : {text}")
    else:
        print(text)
    return 0


def cmd_format(args, config: Config, _home: Path) -> int:
    text = " ".join(args.text)
    result, trace = TextPipeline(config).run_traced(text)
    if args.explain:
        print(trace.render())
        print()
    print(result)
    return 0


def cmd_dict(args, config: Config, home: Path) -> int:
    if args.action == "list":
        if not config.dictionary:
            print("(empty) — add one with: ccvoice dict add \"clod code\" \"Claude Code\"")
        for spoken_form, written in sorted(config.dictionary.items()):
            print(f"{spoken_form:<24} -> {written}")
        return 0
    if args.action == "add":
        written = " ".join(args.written)
        config.add_term(args.spoken, written)
        config.save(home)
        print(f"added: {args.spoken.lower()} -> {written}")
        return 0
    if args.action == "rm":
        removed = config.remove_term(args.spoken)
        config.save(home)
        print("removed" if removed else "not in the dictionary")
        return 0
    return 1


def cmd_history(args, _config: Config, home: Path) -> int:
    store = HistoryStore(home / "history.db")
    try:
        if args.action == "search":
            rows = store.search(
                " ".join(args.query),
                limit=args.limit,
                since=parse_since(args.since) or None,
                model_id=args.model,
                app=args.app,
            )
            print(format_rows(rows))
            return 0
        if args.action == "show":
            record = store.get(args.id)
            if record is None:
                print("no such entry", file=sys.stderr)
                return 1
            print(f"id         {record.id}")
            print(f"when       {record.timestamp}")
            print(f"duration   {record.duration_s:.2f}s")
            print(f"model      {record.model_id}")
            print(f"app        {record.target_app}  (via {record.injection_method})")
            print(f"\nraw:\n{record.raw_text}\n\nclean:\n{record.clean_text}")
            return 0
        if args.action == "export":
            print(f"exported {store.export_jsonl(args.path)} entries to {args.path}")
            return 0
        if args.action == "clear":
            print(f"deleted {store.clear()} entries")
            return 0
        return 1
    finally:
        store.close()


def cmd_stats(args, _config: Config, home: Path) -> int:
    store = HistoryStore(home / "history.db")
    try:
        print(store.stats(parse_since(args.since) or None).render())
    finally:
        store.close()
    return 0


class _PrintInjector:
    """`--no-inject`: show what would have been typed. Handy for a first run."""

    name = "print"

    def available(self) -> bool:
        return True

    def focused_app(self) -> str:
        return "stdout"

    def inject(self, text: str):
        from .ports import InjectionResult

        print(f"\n--- dictated ---\n{text}\n----------------")
        return InjectionResult(True, self.name, target_app="stdout")


__all__ = ["main", "build_parser"]


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
