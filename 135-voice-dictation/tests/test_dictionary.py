"""The custom dictionary, applied during cleanup."""

from ccvoice.config import Config
from ccvoice.pipeline import TextPipeline


def fmt(dictionary, text):
    return TextPipeline(Config(dictionary=dictionary)).run(text)


def test_a_single_word_correction_is_applied():
    assert fmt({"github": "GitHub"}, "push it to github") == "Push it to GitHub."


def test_a_phrase_correction_is_applied():
    assert fmt({"clod code": "Claude Code"}, "open clod code now") == "Open Claude Code now."


def test_the_longest_matching_phrase_wins():
    dictionary = {"code": "Code", "clod code": "Claude Code"}
    assert fmt(dictionary, "clod code rules") == "Claude Code rules."


def test_matching_ignores_the_spoken_casing():
    assert fmt({"github": "GitHub"}, "GITHUB and GitHub and github") == "GitHub and GitHub and GitHub."


def test_a_correction_survives_the_sentence_caser_at_the_start_of_a_sentence():
    assert fmt({"ios": "iOS"}, "ios is fine") == "iOS is fine."


def test_only_whole_tokens_are_replaced():
    assert fmt({"cat": "dog"}, "the category is cat") == "The category is dog."


def test_corrections_apply_after_filler_removal():
    assert fmt({"clod": "Claude"}, "um clod is here") == "Claude is here."


def test_an_empty_dictionary_changes_nothing():
    assert fmt({}, "leave this alone") == "Leave this alone."


def test_config_round_trips_the_dictionary(home):
    config = Config()
    config.add_term("  GitHub  ", " GitHub ")
    config.add_term("clod code", "Claude Code")
    config.save(home)

    reloaded = Config.load(home)
    assert reloaded.dictionary == {"github": "GitHub", "clod code": "Claude Code"}

    assert reloaded.remove_term("GITHUB") is True
    assert reloaded.remove_term("nothing") is False
    reloaded.save(home)
    assert Config.load(home).dictionary == {"clod code": "Claude Code"}


def test_dictionary_entries_reach_a_live_dictation(harness):
    config = Config(live_preview=False, max_recording_s=0, dictionary={"clod": "Claude"})
    h = harness(config=config, text="clod is listening")
    assert h.speak(seconds=1.0).text == "Claude is listening."
