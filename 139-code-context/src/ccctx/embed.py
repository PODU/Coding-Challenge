"""Text embeddings.

Two embedders share one interface (`dim`, `name`, `embed(texts) -> float32[n, dim]`):

* `HashEmbedder` (default, offline, deterministic): a 384-dimensional
  feature-hashed bag of code-aware terms. Identifiers are split on
  camelCase/snake_case, terms are lightly stemmed, code keywords and English
  stopwords are dropped, and a small table of programming concepts maps
  related words onto shared features ("route" ~ "dispatch" ~ "url").
  Unrelated text lands near-orthogonal, so a cosine floor of 0.3 really does
  filter out noise. It is a lexical stand-in for a neural model, not one.

* `OnnxEmbedder`: the real `all-MiniLM-L12-v2` sentence-transformer, run
  locally through onnxruntime when that package, `tokenizers` and a model
  directory are available (`CCCTX_ONNX_MODEL=/path/with/model.onnx+tokenizer.json`).
  This is the same model the Oracle path loads with DBMS_VECTOR.LOAD_ONNX_MODEL.
"""

from __future__ import annotations

import hashlib
import math
import os
import re
from collections import Counter

import numpy as np

DIM = 384

STOPWORDS = set("""
a an the and or not of to in on at by for from with without into onto is are was were be been being
it its this that these those there here as if then else elif when while do does did done can could
should would will shall may might must have has had having i you he she we they them our your their
my me us what which who whom whose how why where all any some each every no nor so than too very just
also only own same such both few more most other about above below again further once out over under
up down off use used using via per get set new let var const def func fn return returns pass none null
nil true false self cls this super import export package from type struct class public private protected
static final void int str string bool float err error ok args kwargs param params value values x y z
i j k n s t v
""".split())

# Concept groups: every member also emits the group's feature.
CONCEPTS = {
    "routing": "route router routing dispatch dispatcher url urls endpoint endpoints path mux rule rules map mapping",
    "request": "request req incoming http inbound query",
    "response": "response resp reply write writer output render",
    "handler": "handler handle handlers view views controller callback hook",
    "config": "config configuration setting settings option options env environment",
    "error": "error errors exception exceptions panic fail failure abort raise",
    "json": "json serialize serializer deserialize marshal unmarshal encode decode dumps loads",
    "template": "template templates jinja render html",
    "session": "session sessions cookie cookies",
    "middleware": "middleware middlewares chain interceptor filter before after",
    "test": "test tests testing assert fixture mock",
    "auth": "auth authentication authorize authorization login password token credential",
    "database": "database db sql query table orm",
    "log": "log logger logging trace debug",
    "context": "context ctx",
    "start": "run start serve server listen launch boot",
    "static": "static file files asset assets",
    "blueprint": "blueprint blueprints",
    "create": "build generate create construct make factory",
}
_CONCEPT_OF: dict[str, list[str]] = {}
_CONCEPT_STEMS: dict[str, list[str]] = {}
for concept, words in CONCEPTS.items():
    for w in words.split():
        _CONCEPT_OF.setdefault(w, []).append("@" + concept)

_WORD = re.compile(r"[A-Za-z][A-Za-z0-9]*")
_CAMEL = re.compile(r"[A-Z]+(?=[A-Z][a-z])|[A-Z]?[a-z]+|[A-Z]+|\d+")


def split_identifier(token: str) -> list[str]:
    parts = []
    for chunk in re.split(r"[_\W]+", token):
        if chunk:
            parts.extend(p.lower() for p in _CAMEL.findall(chunk))
    return parts


def stem(word: str) -> str:
    """A small, consistent suffix stripper (route/routes/routed/routing/router -> rout)."""
    out = word
    for suffix, min_len in (("ations", 7), ("ation", 6), ("ings", 6), ("ing", 5), ("ers", 5),
                            ("ies", 4), ("ed", 4), ("er", 4), ("es", 4), ("s", 3)):
        if word.endswith(suffix) and len(word) >= min_len + 1:
            base = word[: -len(suffix)]
            if suffix == "ies":
                out = base + "y"
            elif suffix == "es" and not base.endswith(("s", "x", "z", "ch", "sh")):
                out = word[:-1]
            elif suffix in ("ing", "ed", "er", "ers", "ings") and len(base) > 2 and base[-1] == base[-2]:
                out = base[:-1]
            else:
                out = base
            break
    if out.endswith("e") and len(out) >= 5:
        out = out[:-1]
    return out


def terms(text: str) -> list[str]:
    """Code-aware terms: split identifiers, drop stopwords, stem, add concepts."""
    out: list[str] = []
    for raw in _WORD.findall(text):
        for part in split_identifier(raw):
            if len(part) < 2 or part in STOPWORDS:
                continue
            st = stem(part)
            out.append(st)
            concepts = _CONCEPT_OF.get(part) or _CONCEPT_STEMS.get(st)
            if concepts:
                out.extend(concepts)
    return out


for _w, _cs in list(_CONCEPT_OF.items()):
    for _c in _cs:
        if _c not in _CONCEPT_STEMS.setdefault(stem(_w), []):
            _CONCEPT_STEMS[stem(_w)].append(_c)


def _bucket(term: str, dim: int) -> tuple[int, float]:
    h = hashlib.blake2b(term.encode(), digest_size=8).digest()
    v = int.from_bytes(h, "little")
    return v % dim, (1.0 if (v >> 63) & 1 else -1.0)


class HashEmbedder:
    name = "hash-v4"

    def __init__(self, dim: int = DIM):
        self.dim = dim
        self._cache: dict[str, tuple[int, float]] = {}

    def _vec(self, text: str) -> np.ndarray:
        vec = np.zeros(self.dim, dtype=np.float32)
        counts = Counter(terms(text))
        for term, c in counts.items():
            if term not in self._cache:
                self._cache[term] = _bucket(term, self.dim)
            idx, sign = self._cache[term]
            weight = 1.0 + math.log(c)
            if term.startswith("@"):
                weight *= 0.6
            vec[idx] += sign * weight
        norm = float(np.linalg.norm(vec))
        return vec / norm if norm else vec

    def embed(self, texts: list[str]) -> np.ndarray:
        if not texts:
            return np.zeros((0, self.dim), dtype=np.float32)
        return np.stack([self._vec(t) for t in texts])


class OnnxEmbedder:  # pragma: no cover - exercised only when the optional stack is installed
    name = "all-MiniLM-L12-v2"

    def __init__(self, model_dir: str):
        import onnxruntime as ort
        from tokenizers import Tokenizer

        self.dim = DIM
        self.tok = Tokenizer.from_file(os.path.join(model_dir, "tokenizer.json"))
        self.tok.enable_truncation(256)
        self.tok.enable_padding()
        self.sess = ort.InferenceSession(os.path.join(model_dir, "model.onnx"))
        self.inputs = {i.name for i in self.sess.get_inputs()}

    def embed(self, texts: list[str]) -> np.ndarray:
        out = []
        for i in range(0, len(texts), 32):
            enc = self.tok.encode_batch(texts[i:i + 32])
            ids = np.array([e.ids for e in enc], dtype=np.int64)
            mask = np.array([e.attention_mask for e in enc], dtype=np.int64)
            feed = {"input_ids": ids, "attention_mask": mask}
            if "token_type_ids" in self.inputs:
                feed["token_type_ids"] = np.zeros_like(ids)
            hidden = self.sess.run(None, feed)[0]
            m = mask[..., None].astype(np.float32)
            pooled = (hidden * m).sum(1) / np.clip(m.sum(1), 1e-9, None)
            pooled /= np.linalg.norm(pooled, axis=1, keepdims=True)
            out.append(pooled.astype(np.float32))
        return np.concatenate(out) if out else np.zeros((0, self.dim), dtype=np.float32)


def get_embedder(name: str | None = None):
    name = name or os.environ.get("CCCTX_EMBEDDER", "hash-v4")
    if name in ("onnx", OnnxEmbedder.name):
        model_dir = os.environ.get("CCCTX_ONNX_MODEL")
        if not model_dir:
            raise RuntimeError("CCCTX_ONNX_MODEL must point at a directory with model.onnx and tokenizer.json")
        return OnnxEmbedder(model_dir)
    return HashEmbedder()


def symbol_text(name: str, qualname: str, kind: str, signature: str, doc: str, source: str, path: str) -> str:
    """The text that represents a symbol for embedding: name-heavy, then docs, then code."""
    name_words = " ".join(split_identifier(qualname))
    body = source[:1000]
    return f"{name_words} {name_words} {name_words} {kind} {signature}\n{doc[:600]}\n{path}\n{body}"


def to_blob(vec: np.ndarray) -> bytes:
    return np.asarray(vec, dtype=np.float32).tobytes()


def from_blob(blob: bytes) -> np.ndarray:
    return np.frombuffer(blob, dtype=np.float32)
