"""Command-line front door for cczipcrack.

Usage::

    cczipcrack <zipfile>                       # validate headers only
    cczipcrack -dict [-w PATH] <zipfile>       # dictionary attack
    cczipcrack -brute -min=N -max=N <zipfile>  # brute-force attack

Exit codes: 0 = found / valid zip, 1 = password not found, 2 = bad usage or
unreadable / non-ZIP input.
"""

from __future__ import annotations

import argparse
import os
import sys
import tempfile
import zipfile
from collections.abc import Iterable, Sequence

from . import __version__, candidates, cracker, reporter, zip_reader
from .candidates import DEFAULT_CHARSET
from .verifier import MissingDependencyError, Verifier

DEFAULT_WORDLIST = os.path.join("wordlists", "smaller.txt")

EXIT_OK = 0
EXIT_NOT_FOUND = 1
EXIT_USAGE = 2


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="cczipcrack",
        description=(
            "Educational ZIP password cracker (Coding Challenge #66). "
            "Use only on archives you own or are authorized to test."
        ),
    )
    parser.add_argument("zipfile", help="Path to the ZIP archive.")

    mode = parser.add_mutually_exclusive_group()
    mode.add_argument(
        "-dict",
        dest="dict_mode",
        action="store_true",
        help="Dictionary attack (use with -w/--wordlist).",
    )
    mode.add_argument(
        "-brute",
        dest="brute_mode",
        action="store_true",
        help="Brute-force attack (use with -min/-max).",
    )

    parser.add_argument(
        "-w",
        "--wordlist",
        default=DEFAULT_WORDLIST,
        help=f"Wordlist path for -dict (default: {DEFAULT_WORDLIST}).",
    )
    parser.add_argument(
        "-min",
        "--min-len",
        dest="min_len",
        type=int,
        default=1,
        help="Minimum candidate length for -brute (default: 1).",
    )
    parser.add_argument(
        "-max",
        "--max-len",
        dest="max_len",
        type=int,
        default=4,
        help="Maximum candidate length for -brute (default: 4).",
    )
    parser.add_argument(
        "--charset",
        default=DEFAULT_CHARSET,
        help="Characters to combine for -brute (default: a-z).",
    )
    parser.add_argument(
        "-j",
        "--jobs",
        type=int,
        default=None,
        help="Worker processes (default: os.cpu_count()).",
    )
    parser.add_argument(
        "--recurse",
        action="store_true",
        help="After success, recurse into a nested ZIP (extension).",
    )
    parser.add_argument(
        "-q",
        "--quiet",
        action="store_true",
        help="Print only result lines.",
    )
    parser.add_argument(
        "--version",
        action="version",
        version=f"%(prog)s {__version__}",
    )
    return parser


def _build_candidates(args: argparse.Namespace) -> Iterable[str]:
    if args.dict_mode:
        return candidates.from_wordlist(args.wordlist)
    return candidates.brute_force(args.min_len, args.max_len, args.charset)


def _emit(message: str, *, quiet: bool = False, force: bool = False) -> None:
    if force or not quiet:
        print(message)


def _run_crack(
    args: argparse.Namespace, path: str, *, out: object = None
) -> int:
    """Run a dict/brute crack against *path*. Returns an exit code."""
    scheme = zip_reader.detect_scheme(path)
    if scheme == "none":
        _emit(reporter.not_encrypted())
        return EXIT_OK
    if scheme == "aes":
        try:
            import pyzipper  # noqa: F401
        except ImportError:
            _emit(reporter.aes_unsupported())
            return EXIT_USAGE

    target = zip_reader.smallest_encrypted_entry(path)
    cand = _build_candidates(args)

    try:
        found = cracker.crack(
            path,
            cand,
            scheme=scheme,
            target=target.name,
            jobs=args.jobs,
        )
    except MissingDependencyError:
        _emit(reporter.aes_unsupported())
        return EXIT_USAGE
    except FileNotFoundError as exc:  # e.g. missing wordlist
        _emit(f"cczipcrack: {exc}", force=True)
        return EXIT_USAGE

    if found is None:
        _emit(reporter.password_not_found(), force=True)
        return EXIT_NOT_FOUND

    _emit(reporter.password_found(found), force=True)

    if args.recurse:
        _recurse(args, path, scheme, target.name, found)
    return EXIT_OK


def _recurse(
    args: argparse.Namespace,
    path: str,
    scheme: str,
    target_name: str,
    password: str,
) -> None:
    """If the entry we just recovered is itself a ZIP, crack that too."""
    try:
        with Verifier(path, scheme, target_name) as verifier:  # type: ignore[arg-type]
            zf = verifier._archive()
            data = zf.read(target_name, pwd=password.encode("utf-8"))
    except Exception:  # pragma: no cover - best-effort extension
        return
    if data[:4] not in (b"PK\x03\x04", b"PK\x05\x06"):
        return
    with tempfile.NamedTemporaryFile(suffix=".zip", delete=False) as tmp:
        tmp.write(data)
        nested_path = tmp.name
    try:
        _emit(f"Nested ZIP detected in {target_name!r}; recursing...",
              quiet=args.quiet)
        if zip_reader.detect_scheme(nested_path) == "none":
            _emit(reporter.not_encrypted(), quiet=args.quiet)
            return
        _run_crack(args, nested_path)
    finally:
        try:
            os.unlink(nested_path)
        except OSError:
            pass


def main(argv: Sequence[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    path = args.zipfile

    if not os.path.exists(path):
        _emit(f"cczipcrack: {path}: no such file", force=True)
        return EXIT_USAGE

    if not zip_reader.is_zip(path):
        _emit(reporter.no_headers(), force=True)
        return EXIT_USAGE

    # No attack flag: just report whether the archive has ZIP headers.
    if not args.dict_mode and not args.brute_mode:
        _emit(reporter.has_headers(), force=True)
        return EXIT_OK

    try:
        return _run_crack(args, path)
    except zipfile.BadZipFile as exc:
        _emit(f"cczipcrack: corrupt archive: {exc}", force=True)
        return EXIT_USAGE
    except KeyboardInterrupt:  # pragma: no cover
        _emit("\ncczipcrack: interrupted", force=True)
        return EXIT_USAGE


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
