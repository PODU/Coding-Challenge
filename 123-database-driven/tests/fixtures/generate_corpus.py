"""Generate the deterministic fixture corpus (~60 tiny source documents).

Run once to (re)create ``tests/fixtures/corpus/``. The documents are plain
prose with clearly capitalized entities so the offline ``ScriptedLLM``
heuristics extract them deterministically. Two extra documents form a
deliberate contradiction pair used by the contradiction test.
"""

from __future__ import annotations

from pathlib import Path

# (entity, field, year, contribution)
PEOPLE = [
    ("Ada Lovelace", "computing", "1843", "the first published algorithm"),
    ("Charles Babbage", "computing", "1837", "the Analytical Engine design"),
    ("Alan Turing", "computing", "1936", "the Turing Machine model"),
    ("Grace Hopper", "computing", "1952", "the first compiler"),
    ("John von Neumann", "computing", "1945", "the stored-program architecture"),
    ("Claude Shannon", "information theory", "1948", "a mathematical theory of communication"),
    ("Geoffrey Hinton", "deep learning", "1986", "the backpropagation algorithm"),
    ("Yann LeCun", "deep learning", "1989", "convolutional neural networks"),
    ("Yoshua Bengio", "deep learning", "2003", "neural language models"),
    ("Frank Rosenblatt", "machine learning", "1958", "the Perceptron"),
    ("Marvin Minsky", "artificial intelligence", "1969", "perceptron limitations"),
    ("John McCarthy", "artificial intelligence", "1956", "the term artificial intelligence"),
    ("Edsger Dijkstra", "algorithms", "1959", "the shortest path algorithm"),
    ("Donald Knuth", "algorithms", "1968", "the Art of Computer Programming"),
    ("Barbara Liskov", "programming languages", "1974", "data abstraction"),
    ("Tim Berners Lee", "networking", "1989", "the World Wide Web"),
    ("Vint Cerf", "networking", "1974", "the Transmission Control Protocol"),
    ("Dennis Ritchie", "programming languages", "1972", "the C programming language"),
    ("Ken Thompson", "operating systems", "1969", "the Unix operating system"),
    ("Linus Torvalds", "operating systems", "1991", "the Linux kernel"),
]

ORGS = [
    ("Bell Labs", "research", "the transistor"),
    ("Xerox Parc", "research", "the graphical user interface"),
    ("Stanford University", "education", "early artificial intelligence research"),
    ("Massachusetts Institute of Technology", "education", "the Lisp programming language"),
    ("Google Research", "industry", "the Transformer architecture"),
]


def build_docs() -> dict[str, str]:
    docs: dict[str, str] = {}
    i = 0
    for name, field, year, contribution in PEOPLE:
        i += 1
        docs[f"{i:02d}_{name.split()[-1].lower()}.txt"] = (
            f"{name} was a pioneer of {field}. "
            f"{name} is credited with {contribution} in {year}. "
            f"The work of {name} influenced the study of {field}."
        )
    for name, kind, contribution in ORGS:
        i += 1
        slug = name.split()[0].lower()
        docs[f"{i:02d}_{slug}.txt"] = (
            f"{name} was an important {kind} organization. "
            f"{name} is known for {contribution}. "
            f"Researchers at {name} advanced the field significantly."
        )
    # Cross-link topic documents to enrich the wiki.
    extra = [
        ("Deep Learning", "Deep Learning relies on neural networks. "
         "Geoffrey Hinton and Yann LeCun advanced Deep Learning. "
         "Deep Learning powers modern computer vision."),
        ("The Perceptron", "The Perceptron was an early neural model. "
         "Frank Rosenblatt built the Perceptron. "
         "Marvin Minsky analyzed the Perceptron limitations."),
        ("The Analytical Engine", "The Analytical Engine was a mechanical computer. "
         "Charles Babbage designed the Analytical Engine. "
         "Ada Lovelace wrote programs for the Analytical Engine."),
        ("The Transformer", "The Transformer is a neural architecture. "
         "Google Research introduced the Transformer. "
         "The Transformer uses an attention mechanism."),
    ]
    for title, body in extra:
        i += 1
        slug = title.split()[-1].lower()
        docs[f"{i:02d}_{slug}.txt"] = body

    # Pad to ~60 with concept notes.
    concepts = [
        "Recursion", "Compilers", "Cryptography", "Databases", "Concurrency",
        "Virtual Memory", "Garbage Collection", "Type Systems", "Graph Theory",
        "Boolean Logic", "Floating Point", "Cache Coherence", "Distributed Systems",
        "Public Key Cryptography", "Hash Tables", "Sorting Algorithms",
        "Finite Automata", "Lambda Calculus", "Operating Systems", "Network Protocols",
        "Machine Learning", "Reinforcement Learning", "Computer Vision",
        "Natural Language Processing", "Computer Architecture", "Information Retrieval",
        "Formal Verification", "Quantum Computing", "Computational Complexity",
    ]
    for c in concepts:
        i += 1
        slug = c.split()[-1].lower()
        docs[f"{i:02d}_{slug}.txt"] = (
            f"{c} is a foundational topic in computer science. "
            f"{c} appears throughout the study of algorithms and systems. "
            f"Understanding {c} is essential for engineers."
        )
    return docs


# Deliberate contradiction pair (NOT part of the main corpus dir; written
# separately so tests can ingest them in isolation).
CONTRADICTION_A = (
    "Ada Lovelace was born in London. "
    "Ada Lovelace was born in 1815. "
    "Ada Lovelace worked on the Analytical Engine."
)
CONTRADICTION_B = (
    "Ada Lovelace was born in London. "
    "Ada Lovelace was born in 1900. "
    "Ada Lovelace worked on the Analytical Engine."
)


def main() -> None:
    out = Path(__file__).parent / "corpus"
    out.mkdir(parents=True, exist_ok=True)
    for name, body in build_docs().items():
        (out / name).write_text(body + "\n", encoding="utf-8")
    print(f"wrote {len(build_docs())} docs to {out}")


if __name__ == "__main__":
    main()
