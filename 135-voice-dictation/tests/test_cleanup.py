"""Filler removal, stutter collapse, punctuation and sentence casing."""

import pytest

from ccvoice.cleanup import collapse_repeats, remove_fillers
from ccvoice.config import Config
from ccvoice.pipeline import TextPipeline, word_count
from ccvoice.tokens import render, tokenize


def clean(text, **overrides):
    return TextPipeline(Config(**overrides)).run(text)


@pytest.mark.parametrize(
    "spoken, expected",
    [
        ("um hello there", "Hello there."),
        ("uh so we ship uh tomorrow", "So we ship tomorrow."),
        ("erm i think mhm it works", "I think it works."),
        ("this is, um, the plan", "This is, the plan."),
    ],
)
def test_filler_words_are_removed(spoken, expected):
    assert clean(spoken) == expected


def test_filler_phrases_are_removed():
    assert clean("you know we should i mean just ship it") == "We should just ship it."


def test_stutters_are_collapsed():
    assert clean("the the plan is is ready") == "The plan is ready."


def test_words_that_legitimately_double_are_kept():
    assert clean("i had had enough") == "I had had enough."


def test_collapse_can_be_switched_off():
    assert clean("the the plan", collapse_repeats=False) == "The the plan."


def test_sentence_casing_applies_after_every_full_stop():
    assert clean("hello there. how are you? fine!") == "Hello there. How are you? Fine!"


def test_the_pronoun_i_is_always_capitalised():
    assert clean("i think i'm right and i'll prove it") == "I think I'm right and I'll prove it."


def test_a_missing_terminal_stop_is_added_once():
    assert clean("we are done") == "We are done."
    assert clean("we are done.") == "We are done."
    assert clean("are we done?") == "Are we done?"


def test_a_trailing_comma_becomes_a_full_stop():
    assert clean("that is all comma") == "That is all."


def test_existing_capitals_are_left_alone():
    assert clean("the API is ready") == "The API is ready."


def test_cleanup_can_be_disabled_wholesale():
    assert clean("um the the plan", cleanup=False, sentence_case=False) == "um the the plan"


def test_remove_fillers_is_a_pure_token_transform():
    tokens = tokenize("um, hello")
    assert render(remove_fillers(tokens, ["um"], [])) == "hello"
    assert render(tokens) == "um, hello"  # the input list is untouched


def test_collapse_repeats_keeps_punctuation_between_repeats():
    assert render(collapse_repeats(tokenize("no, no, we stop"))) == "no, no, we stop"


def test_word_count_ignores_stray_punctuation():
    assert word_count("Hello, world - ok?") == 3


def test_cleanup_is_stable_when_run_twice():
    once = clean("um so we ship it")
    assert clean(once) == once
