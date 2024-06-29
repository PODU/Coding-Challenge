"""Check candidate passwords across several processes and stop at the first
match. Candidates are sent to the pool in chunks to cut messaging overhead.
jobs == 1 runs in the current process.
"""

from __future__ import annotations

import itertools
import multiprocessing as mp
import os
from collections.abc import Iterable, Iterator

from .verifier import Verifier
from .zip_reader import Scheme

DEFAULT_CHUNK_SIZE = 512

# One verifier per worker. It is built in the worker rather than passed in
# because an open ZIP file can't be pickled.
_WORKER_VERIFIER: Verifier | None = None


def _init_worker(path: str, scheme: Scheme, target: str) -> None:
    global _WORKER_VERIFIER
    _WORKER_VERIFIER = Verifier(path, scheme, target)


def _check_chunk(chunk: list[str]) -> str | None:
    verifier = _WORKER_VERIFIER
    assert verifier is not None
    for password in chunk:
        if verifier.verify(password):
            return password
    return None


def _chunked(iterable: Iterable[str], size: int) -> Iterator[list[str]]:
    it = iter(iterable)
    while True:
        chunk = list(itertools.islice(it, size))
        if not chunk:
            return
        yield chunk


def crack(
    path: str,
    candidates: Iterable[str],
    *,
    scheme: Scheme,
    target: str,
    jobs: int | None = None,
    chunk_size: int = DEFAULT_CHUNK_SIZE,
) -> str | None:
    """Return the first candidate that verifies, or ``None``. *jobs* defaults
    to os.cpu_count()."""
    if jobs is None:
        jobs = os.cpu_count() or 1
    if jobs < 1:
        raise ValueError("jobs must be >= 1")

    if jobs == 1:
        return _crack_single(path, candidates, scheme=scheme, target=target)
    return _crack_pool(
        path,
        candidates,
        scheme=scheme,
        target=target,
        jobs=jobs,
        chunk_size=chunk_size,
    )


def _crack_single(
    path: str,
    candidates: Iterable[str],
    *,
    scheme: Scheme,
    target: str,
) -> str | None:
    with Verifier(path, scheme, target) as verifier:
        for password in candidates:
            if verifier.verify(password):
                return password
    return None


def _crack_pool(
    path: str,
    candidates: Iterable[str],
    *,
    scheme: Scheme,
    target: str,
    jobs: int,
    chunk_size: int,
) -> str | None:
    pool = mp.Pool(
        processes=jobs,
        initializer=_init_worker,
        initargs=(path, scheme, target),
    )
    try:
        chunks = _chunked(candidates, chunk_size)
        for result in pool.imap_unordered(_check_chunk, chunks):
            if result is not None:
                return result
        return None
    finally:
        pool.terminate()
        pool.join()
