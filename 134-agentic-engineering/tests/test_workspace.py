import pytest

from agentloop.workspace import parse_file_blocks, write_files
from conftest import CALC_FULL


def test_parse_single_block():
    files = parse_file_blocks(CALC_FULL)
    assert list(files) == ["calc.py"]
    assert "if b == 0:" in files["calc.py"]
    assert files["calc.py"].endswith("\n")


def test_parse_multiple_blocks_with_prose():
    text = ("some prose before\n"
            "=== FILE: a.py ===\nx = 1\n=== END FILE ===\n"
            "between\n"
            "=== FILE: sub/b.py ===\ny = 2\n=== END FILE ===\n")
    files = parse_file_blocks(text)
    assert files == {"a.py": "x = 1\n", "sub/b.py": "y = 2\n"}


def test_parse_no_blocks():
    assert parse_file_blocks("no code here") == {}


def test_write_files_creates_dirs(tmp_path):
    written = write_files(tmp_path, {"pkg/mod.py": "z = 3\n"})
    assert written == ["pkg/mod.py"]
    assert (tmp_path / "pkg" / "mod.py").read_text() == "z = 3\n"


def test_write_files_blocks_escape(tmp_path):
    ws = tmp_path / "ws"
    ws.mkdir()
    with pytest.raises(ValueError, match="outside workspace"):
        write_files(ws, {"../evil.py": "boom\n"})
