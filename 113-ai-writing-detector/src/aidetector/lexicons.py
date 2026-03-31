"""Phrase lists, weights and caps used by the detectors.

All the tuning knobs live here. The detector caps add up to 100.
"""

from __future__ import annotations

# Detector caps (sum to 100)

CAP_SIGNATURE = 20.0
CAP_STRUCTURAL = 20.0
CAP_VAGUE = 18.0
CAP_EMPHASIS = 17.0
CAP_LINGUISTIC = 25.0

# Signature words and phrases

SIGNATURE_PHRASES: tuple[str, ...] = (
    "delve into",
    "delve deeper",
    "navigate the complexities",
    "navigate the landscape",
    "in the realm of",
    "in the ever-evolving",
    "ever-evolving landscape",
    "it is important to note",
    "when it comes to",
    "at the end of the day",
    "the world of",
    "a testament to",
    "stand the test of time",
    "pave the way",
    "shed light on",
    "a deep dive",
    "unlock the potential",
    "harness the power",
    "tapestry of",
    "treasure trove",
)

SIGNATURE_WORDS: tuple[str, ...] = (
    "delve",
    "robust",
    "innovative",
    "navigate",
    "leverage",
    "seamless",
    "holistic",
    "multifaceted",
    "underscore",
    "underscores",
    "myriad",
    "plethora",
    "intricate",
    "nuanced",
    "pivotal",
    "paramount",
    "comprehensive",
    "vibrant",
    "bustling",
    "realm",
    "landscape",
    "foster",
    "facilitate",
    "utilize",
    "embark",
    "elevate",
    "showcasing",
    "encompasses",
    "evolving",
)

WEIGHT_SIGNATURE_WORD = 3.0
WEIGHT_SIGNATURE_PHRASE = 4.0

# Structural patterns

# Outline-style conclusion openers.
OUTLINE_CONCLUSIONS: tuple[str, ...] = (
    "in conclusion",
    "to conclude",
    "to summarize",
    "to summarise",
    "in summary",
    "all in all",
    "in essence",
    "to sum up",
    "ultimately,",
    "in the final analysis",
)

WEIGHT_RULE_OF_THREE = 4.0
WEIGHT_NEGATIVE_PARALLELISM = 5.0
WEIGHT_OUTLINE_CONCLUSION = 5.0
WEIGHT_FALSE_RANGE = 4.0

# Vague language

VAGUE_ATTRIBUTIONS: tuple[str, ...] = (
    "studies show",
    "studies have shown",
    "research suggests",
    "research indicates",
    "experts say",
    "experts agree",
    "it is widely believed",
    "it is widely accepted",
    "many believe",
    "many argue",
    "some argue",
    "it is often said",
    "it is generally accepted",
)

SUPERFICIAL_ANALYSIS: tuple[str, ...] = (
    "plays a crucial role",
    "plays a vital role",
    "plays a key role",
    "plays a significant role",
    "is an important aspect",
    "is a key factor",
    "it is worth noting",
    "key to understanding",
    "a wide range of",
    "a variety of factors",
    "on so many levels",
)

OVERGENERALIZATION: tuple[str, ...] = (
    "in today's world",
    "in todays world",
    "in the modern era",
    "in the modern world",
    "throughout history",
    "since the dawn of time",
    "in our fast-paced world",
    "in our increasingly",
    "more than ever before",
    "now more than ever",
    "in an age where",
)

WEIGHT_VAGUE_ITEM = 3.0
CAP_VAGUE_CATEGORY = 6.0  # per-category cap (3 categories -> 18 total)

# Excessive emphasis

PROMOTIONAL_TERMS: tuple[str, ...] = (
    "game-changer",
    "game changer",
    "cutting-edge",
    "state-of-the-art",
    "revolutionary",
    "groundbreaking",
    "world-class",
    "best-in-class",
    "must-have",
    "next-level",
    "unparalleled",
    "unleash",
    "supercharge",
    "empower",
    "transformative",
    "unrivaled",
    "unrivalled",
)

INTENSIFIERS: tuple[str, ...] = (
    "very",
    "really",
    "extremely",
    "incredibly",
    "remarkably",
    "truly",
    "absolutely",
    "highly",
    "vastly",
    "immensely",
)

# Elegant-variation synonym groups: cycling through synonyms for one concept.
ELEGANT_VARIATION_GROUPS: tuple[tuple[str, ...], ...] = (
    ("important", "crucial", "vital", "essential", "pivotal", "paramount"),
    ("show", "demonstrate", "illustrate", "highlight", "underscore", "reveal"),
    ("big", "large", "vast", "immense", "substantial", "considerable"),
    ("use", "utilize", "employ", "leverage", "harness"),
    ("good", "excellent", "exceptional", "outstanding", "remarkable"),
)

WEIGHT_PROMOTIONAL = 3.0
WEIGHT_UNDUE_EMPHASIS = 2.0
WEIGHT_ELEGANT_VARIATION = 3.0
CAP_EMPHASIS_CATEGORY = 7.0

# Linguistic factors

TRANSITION_WORDS: tuple[str, ...] = (
    "however",
    "moreover",
    "furthermore",
    "additionally",
    "consequently",
    "therefore",
    "nevertheless",
    "nonetheless",
    "subsequently",
    "accordingly",
    "thus",
    "hence",
    "meanwhile",
    "likewise",
    "similarly",
    "indeed",
    "notably",
    "importantly",
    "specifically",
    "ultimately",
)

# Contraction markers (their *absence* is mildly AI-leaning).
CONTRACTIONS: tuple[str, ...] = (
    "n't",
    "'re",
    "'ll",
    "'ve",
    "'m",
    "'d",
    "it's",
    "that's",
    "i'm",
    "you're",
    "we're",
    "they're",
    "don't",
    "can't",
    "won't",
    "isn't",
)

# A small bundled common-word list. Tokens outside it count as "rare".
COMMON_WORDS: frozenset[str] = frozenset(
    """
    the be to of and a in that have i it for not on with he as you do at this but his by
    from they we say her she or an will my one all would there their what so up out if about
    who get which go me when make can like time no just him know take people into year your
    good some could them see other than then now look only come its over think also back
    after use two how our work first well way even new want because any these give day most us
    is are was were been being am has had did does doing more very much many such own same
    each few both little here where why again once under over again then while during before
    after above below between through against off down up out in on at by for with about into
    over under
    text essay write writing read story book word words people person world life thing things
    help need feel really pretty kind lot bit going got went said told asked tried tried
    school house home car food water money family friend friends mom dad sister brother
    happy sad great big small long short hard easy fun nice cool weird funny
    yesterday today tomorrow morning night week month place around always never sometimes
    actually maybe probably honestly basically literally anyway though stuff guess wanted
    """.split()
)

# Per-factor maximum point contributions (sum ~= 26, capped at 25).
MAX_LEXICAL = 3.0
MAX_SENT_VARIATION = 5.0
MAX_PASSIVE = 4.0
MAX_TRANSITION = 4.0
MAX_FLESCH_KINCAID = 3.0
MAX_PUNCTUATION = 3.0
MAX_RARE_WORD = 4.0

# Linguistic factors are scaled by this for short documents.
SHORT_TEXT_WEIGHT = 0.5
