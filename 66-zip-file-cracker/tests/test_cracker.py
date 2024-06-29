"""Orchestrator + candidate generators (challenge Steps 2, 3, 4)."""

from __future__ import annotations

from pathlib import Path

from cczipcrack import candidates, cracker


def test_dictionary_success(zipcrypto_zip: Path, tiny_wordlist: Path) -> None:
    found = cracker.crack(
        str(zipcrypto_zip),
        candidates.from_wordlist(str(tiny_wordlist)),
        scheme="zipcrypto",
        target="cc.txt",
        jobs=1,
    )
    assert found == "test"


def test_dictionary_miss(zipcrypto_zip: Path, nomatch_wordlist: Path) -> None:
    found = cracker.crack(
        str(zipcrypto_zip),
        candidates.from_wordlist(str(nomatch_wordlist)),
        scheme="zipcrypto",
        target="cc.txt",
        jobs=1,
    )
    assert found is None


def test_brute_force_success(zipcrypto_zip: Path) -> None:
    # Restrict the charset to keep the space tiny but still contain 'test'.
    found = cracker.crack(
        str(zipcrypto_zip),
        candidates.brute_force(1, 4, charset="set"),
        scheme="zipcrypto",
        target="cc.txt",
        jobs=1,
    )
    assert found == "test"


def test_brute_force_miss(zipcrypto_zip: Path) -> None:
    found = cracker.crack(
        str(zipcrypto_zip),
        candidates.brute_force(1, 3, charset="set"),  # too short for 'test'
        scheme="zipcrypto",
        target="cc.txt",
        jobs=1,
    )
    assert found is None


def test_concurrency_parity(zipcrypto_zip: Path, tiny_wordlist: Path) -> None:
    # Correctness must not depend on worker count.
    one = cracker.crack(
        str(zipcrypto_zip),
        candidates.from_wordlist(str(tiny_wordlist)),
        scheme="zipcrypto",
        target="cc.txt",
        jobs=1,
    )
    many = cracker.crack(
        str(zipcrypto_zip),
        candidates.from_wordlist(str(tiny_wordlist)),
        scheme="zipcrypto",
        target="cc.txt",
        jobs=2,
        chunk_size=1,
    )
    assert one == many == "test"


def test_brute_force_generator_ordering() -> None:
    gen = candidates.brute_force(1, 2, charset="ab")
    assert list(gen) == ["a", "b", "aa", "ab", "ba", "bb"]


def test_wordlist_streams_and_strips(tmp_path: Path) -> None:
    wl = tmp_path / "wl.txt"
    wl.write_text("alpha\n\nbeta\r\n", encoding="utf-8")
    assert list(candidates.from_wordlist(str(wl))) == ["alpha", "beta"]
