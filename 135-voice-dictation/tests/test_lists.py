"""Ordinal runs and spoken bullets become formatted lists."""

from ccvoice.config import Config
from ccvoice.listfmt import apply_lists
from ccvoice.pipeline import TextPipeline
from ccvoice.tokens import render, tokenize


def fmt(text, **overrides):
    return TextPipeline(Config(**overrides)).run(text)


def test_an_ordinal_run_becomes_a_numbered_list():
    assert fmt("first check the logs second restart the service third tell the team") == (
        "1. Check the logs\n2. Restart the service\n3. Tell the team"
    )


def test_an_intro_clause_survives_and_gains_a_colon():
    assert fmt("here are the steps first check the logs second restart it") == (
        "Here are the steps:\n1. Check the logs\n2. Restart it"
    )


def test_a_one_word_intro_gets_no_colon():
    """"um so" is not an introduction — it is what is left after the filler went."""
    assert fmt("um so first check the logs second restart it") == (
        "So\n1. Check the logs\n2. Restart it"
    )


def test_the_list_style_is_configurable():
    assert fmt("first milk second eggs", list_style="bulleted") == "- Milk\n- Eggs"
    assert fmt("first milk second eggs", list_style="bulleted", bullet_marker="*") == "* Milk\n* Eggs"


def test_spoken_bullet_points_make_a_bulleted_list():
    assert fmt("bullet point milk bullet point eggs bullet point bread") == "- Milk\n- Eggs\n- Bread"


def test_a_lone_ordinal_is_not_a_list():
    assert fmt("first we should talk about it") == "First we should talk about it."


def test_a_non_ascending_ordinal_is_not_a_list():
    assert fmt("i had second thoughts about the first draft") == (
        "I had second thoughts about the first draft."
    )


def test_ordinals_spoken_with_commas_still_split_correctly():
    assert fmt("first, wake up. second, drink coffee.") == "1. Wake up\n2. Drink coffee"


def test_a_paragraph_break_closes_the_list():
    assert fmt("first milk second eggs new paragraph that is the whole list") == (
        "1. Milk\n2. Eggs\n\nThat is the whole list."
    )


def test_list_items_do_not_get_a_terminal_full_stop():
    assert fmt("first milk second eggs").endswith("Eggs")


def test_manual_line_breaks_beat_the_list_detector():
    """Saying "new line" between the ordinals means the speaker is formatting by hand."""
    assert fmt("first line new line second line") == "First line\nSecond line."


def test_a_break_after_the_run_does_not_disable_the_list():
    assert fmt("first milk second eggs new line done") == "1. Milk\n2. Eggs\nDone."


def test_lists_can_be_switched_off():
    assert fmt("first milk second eggs", detect_lists=False) == "First milk second eggs."


def test_apply_lists_is_a_pure_transform():
    tokens = tokenize("first a second b")
    assert render(apply_lists(tokens)) == "1. a\n2. b"
    assert render(tokens) == "first a second b"


def test_ordinals_beyond_third_keep_numbering():
    text = fmt("first a second b third c fourth d fifth e")
    assert text.splitlines() == ["1. A", "2. B", "3. C", "4. D", "5. E"]
