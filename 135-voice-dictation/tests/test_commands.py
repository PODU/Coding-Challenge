"""Spoken formatting commands become real punctuation and breaks."""

import pytest

from ccvoice.commands import apply_commands, describe
from ccvoice.config import Config
from ccvoice.pipeline import TextPipeline
from ccvoice.tokens import render, tokenize


def spoken(text, **overrides):
    return TextPipeline(Config(**overrides)).run(text)


def test_new_line_inserts_a_real_break():
    assert spoken("first line new line second line") == "First line\nSecond line."


def test_new_paragraph_inserts_a_blank_line():
    assert spoken("intro new paragraph body") == "Intro\n\nBody."


@pytest.mark.parametrize(
    "phrase, mark",
    [
        ("period", "."),
        ("full stop", "."),
        ("comma", ","),
        ("question mark", "?"),
        ("exclamation mark", "!"),
        ("colon", ":"),
        ("semicolon", ";"),
    ],
)
def test_punctuation_words_attach_to_the_previous_word(phrase, mark):
    out = render(apply_commands(tokenize(f"yes {phrase} no")))
    assert out.startswith(f"yes{mark}")


def test_quotes_and_parens_get_their_spacing_right():
    assert spoken("she said open quote ship it close quote today") == 'She said "ship it" today.'
    assert spoken("the fix open paren finally close paren landed") == "The fix (finally) landed."


def test_dash_joins_the_words_either_side():
    assert spoken("a well dash known bug") == "A well-known bug."


def test_literally_escapes_a_command():
    assert spoken("say literally period at the end") == "Say period at the end."
    assert spoken("literally new paragraph is the command") == "New paragraph is the command."


def test_commands_are_matched_longest_first():
    tokens = apply_commands(tokenize("stop full stop go"))
    assert render(tokens) == "stop. go"


def test_commands_can_be_switched_off():
    assert spoken("hello new line world", spoken_commands=False) == "Hello new line world."


def test_break_never_leaves_a_dangling_space_or_leading_blank_line():
    assert spoken("new line hello new line new line world") == "Hello\nWorld."


def test_tab_is_inserted_literally():
    assert render(apply_commands(tokenize("name tab value"))) == "name\tvalue"


def test_describe_lists_every_command_for_the_help_screen():
    rows = dict(describe())
    assert rows["new line"] == "\\n"
    assert rows["question mark"] == "?"
    assert len(rows) >= 15
