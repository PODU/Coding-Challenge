"""Structural code parsing: split files at function, method and class boundaries.

Python is parsed with the standard-library `ast` module, which is exact.
Go, JavaScript, TypeScript, Java and Rust use a lightweight structural
parser: strings and comments are first blanked out (so braces inside them
cannot confuse anything), declarations are found with per-language regular
expressions, and each declaration's body is located by brace matching.
For well-formed code this finds the same boundaries tree-sitter would, with
no native dependencies. Other languages, and files that fail to parse, fall
back to 60-line windows with a 10-line overlap.
"""

from __future__ import annotations

import ast
import bisect
import re
from dataclasses import dataclass, field
from pathlib import PurePosixPath

WINDOW = 60
OVERLAP = 10


@dataclass
class Symbol:
    name: str
    qualname: str
    kind: str  # function, method, class, struct, interface, trait, enum, type, chunk
    start_line: int  # 1-based, inclusive
    end_line: int
    source: str
    signature: str = ""
    doc: str = ""
    calls: list[str] = field(default_factory=list)
    parent: str = ""


@dataclass
class ParsedFile:
    language: str
    symbols: list[Symbol]
    imports: list[str] = field(default_factory=list)
    module_doc: str = ""
    parser: str = "structural"  # structural | ast | fallback


def parse(path: str, text: str, language: str) -> ParsedFile:
    try:
        if language == "python":
            parsed = _parse_python(text)
        elif language in BRACE_LANGS:
            parsed = _parse_brace(text, language)
        else:
            parsed = None
    except (SyntaxError, ValueError, RecursionError):
        parsed = None
    if parsed is None or not parsed.symbols:
        imports = parsed.imports if parsed else []
        doc = parsed.module_doc if parsed else ""
        return ParsedFile(language, window_chunks(path, text, language), imports, doc, "fallback")
    parsed.language = language
    return parsed


_HEADING = re.compile(r"^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$")


def window_chunks(path: str, text: str, language: str = "text") -> list[Symbol]:
    lines = text.splitlines()
    if not lines:
        return []
    base = PurePosixPath(path).name
    out = []
    start = 0
    step = WINDOW - OVERLAP
    while True:
        end = min(start + WINDOW, len(lines))
        body = "\n".join(lines[start:end])
        if body.strip():
            label = f"{base}:{start + 1}-{end}"
            doc = ""
            if language == "markdown":
                for line in lines[start:end]:
                    m = _HEADING.match(line)
                    if m:
                        doc = m.group(1)
                        break
            out.append(Symbol(label, label, "chunk", start + 1, end, body, doc=doc))
        if end >= len(lines):
            break
        start += step
    return out


def _py_calls(nodes, decorators=()) -> list[str]:
    seen: dict[str, None] = {}
    for d in decorators:  # `@login_required` applies (calls) login_required
        if isinstance(d, ast.Name):
            seen.setdefault(d.id)
        elif isinstance(d, ast.Attribute):
            seen.setdefault(d.attr)
    for root in nodes:
        for node in ast.walk(root):
            if isinstance(node, ast.Call):
                f = node.func
                if isinstance(f, ast.Name):
                    seen.setdefault(f.id)
                elif isinstance(f, ast.Attribute):
                    seen.setdefault(f.attr)
    return list(seen)


def _py_signature(node) -> str:
    if isinstance(node, ast.ClassDef):
        bases = ", ".join(ast.unparse(b) for b in node.bases)
        return f"class {node.name}({bases})" if bases else f"class {node.name}"
    prefix = "async def" if isinstance(node, ast.AsyncFunctionDef) else "def"
    sig = f"{prefix} {node.name}({ast.unparse(node.args)})"
    if node.returns is not None:
        sig += f" -> {ast.unparse(node.returns)}"
    return sig


def _parse_python(text: str) -> ParsedFile:
    tree = ast.parse(text)
    lines = text.splitlines()
    symbols: list[Symbol] = []

    def source_of(node) -> tuple[int, int, str]:
        start = min([node.lineno] + [d.lineno for d in getattr(node, "decorator_list", [])])
        end = node.end_lineno or node.lineno
        return start, end, "\n".join(lines[start - 1:end])

    def visit(body, parent: str, in_class: bool):
        for node in body:
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                start, end, src = source_of(node)
                qual = f"{parent}.{node.name}" if parent else node.name
                symbols.append(Symbol(
                    node.name, qual, "method" if in_class else "function", start, end, src,
                    _py_signature(node), ast.get_docstring(node) or "",
                    _py_calls(node.body + node.decorator_list, node.decorator_list), parent))
            elif isinstance(node, ast.ClassDef):
                start, end, src = source_of(node)
                qual = f"{parent}.{node.name}" if parent else node.name
                class_level = [n for n in node.body
                               if not isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef))]
                symbols.append(Symbol(
                    node.name, qual, "class", start, end, src, _py_signature(node),
                    ast.get_docstring(node) or "",
                    _py_calls(class_level + node.bases + node.decorator_list, node.decorator_list), parent))
                visit(node.body, qual, True)
            elif isinstance(node, (ast.If, ast.Try)) and not parent:
                # Top-level `if TYPE_CHECKING:` / `try: import` blocks can hold defs.
                visit(node.body, parent, in_class)
                visit(getattr(node, "orelse", []), parent, in_class)

    visit(tree.body, "", False)

    imports: list[str] = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.extend(a.name for a in node.names)
        elif isinstance(node, ast.ImportFrom):
            imports.append("." * node.level + (node.module or ""))
    return ParsedFile("python", symbols, list(dict.fromkeys(imports)),
                      ast.get_docstring(tree) or "", "ast")


BRACE_LANGS = {"go", "javascript", "typescript", "java", "rust"}

KEYWORDS = {
    "if", "for", "while", "switch", "catch", "return", "func", "function", "new", "else",
    "do", "try", "case", "throw", "typeof", "sizeof", "await", "yield", "match", "loop",
    "fn", "impl", "where", "select", "go", "defer", "range", "struct", "interface",
    "synchronized", "super", "this", "self", "Self", "import", "export",
    "assert", "with", "in", "of", "as", "instanceof", "delete", "void", "make", "len",
    "cap", "append", "panic", "new", "copy", "close", "string", "int", "int64", "int32",
    "uint", "uint64", "uint32", "byte", "rune", "float64", "float32", "bool", "error",
    "Some", "Ok", "Err", "Box", "Vec", "require",
}

CONTAINER_KINDS = {"class", "interface", "trait", "impl", "enum"}
_IDENT = r"[A-Za-z_$][\w$]*"
_MODS_JS = r"(?:(?:public|private|protected|static|async|readonly|override|abstract|get|set)[ \t]+)*"
_MODS_JAVA = r"(?:(?:public|private|protected|static|final|abstract|synchronized|native|default|strictfp|sealed)[ \t]+)*"
_PUB_RS = r"(?:pub(?:\([^)\n]*\))?[ \t]+)?"

# (regex, kind, needs_container). Named groups: name, optional recv/kw.
DECLS: dict[str, list[tuple[re.Pattern, str, bool]]] = {
    "go": [
        (re.compile(r"^func[ \t]*(?:\((?P<recv>[^)]*)\)[ \t]*)?(?P<name>\w+)", re.M), "function", False),
        (re.compile(r"^type[ \t]+(?P<name>\w+)(?:\[[^\]\n]*\])?[ \t]+(?P<kw>struct|interface)\b", re.M), "struct", False),
        (re.compile(r"^type[ \t]+(?P<name>\w+)\b", re.M), "type", False),
    ],
    "javascript": [
        (re.compile(rf"^[ \t]*(?:export[ \t]+)?(?:default[ \t]+)?(?:async[ \t]+)?function\b[ \t]*\*?[ \t]*(?P<name>{_IDENT})", re.M), "function", False),
        (re.compile(rf"^[ \t]*(?:export[ \t]+)?(?:default[ \t]+)?(?:abstract[ \t]+)?class[ \t]+(?P<name>{_IDENT})", re.M), "class", False),
        (re.compile(rf"^[ \t]*(?:export[ \t]+)?(?:const|let|var)[ \t]+(?P<name>{_IDENT})[ \t]*(?::[^=\n]+)?=[ \t]*(?:async[ \t]+)?(?:\([^)]*\)|{_IDENT})[ \t]*(?::[^=\n]+)?=>", re.M), "arrow", False),
        (re.compile(rf"^[ \t]*(?:export[ \t]+)?(?:const|let|var)[ \t]+(?P<name>{_IDENT})[ \t]*=[ \t]*(?:async[ \t]+)?function\b", re.M), "function", False),
        (re.compile(rf"^[ \t]+{_MODS_JS}\*?(?P<name>#?{_IDENT})[ \t]*(?:<[^>\n]*>)?\([^)\n]*\)[ \t]*(?::[^{{\n]+)?\{{", re.M), "method", True),
    ],
    "java": [
        (re.compile(rf"^[ \t]*{_MODS_JAVA}(?P<kw>class|interface|enum|record|@interface)[ \t]+(?P<name>\w+)", re.M), "class", False),
        (re.compile(rf"^[ \t]*{_MODS_JAVA}(?:<[^>\n]+>[ \t]+)?(?:[\w.$]+(?:<[^\n(]*?>)?(?:\[\])*[ \t]+)?(?P<name>\w+)[ \t]*\(", re.M), "method", True),
    ],
    "rust": [
        (re.compile(rf"^[ \t]*{_PUB_RS}(?:(?:const|async|unsafe|extern(?:[ \t]+\"[^\"\n]*\")?)[ \t]+)*fn[ \t]+(?P<name>\w+)", re.M), "function", False),
        (re.compile(rf"^[ \t]*{_PUB_RS}(?P<kw>struct|enum|trait|union)[ \t]+(?P<name>\w+)", re.M), "struct", False),
        (re.compile(r"^[ \t]*(?:unsafe[ \t]+)?impl\b(?:[ \t]*<[^{\n]*?>)?[ \t]+(?:(?P<trait>[\w:]+(?:<[^{\n]*?>)?)[ \t]+for[ \t]+)?(?P<name>\w+)", re.M), "impl", False),
    ],
}
DECLS["typescript"] = DECLS["javascript"] + [
    (re.compile(r"^[ \t]*(?:export[ \t]+)?(?:declare[ \t]+)?interface[ \t]+(?P<name>\w+)", re.M), "interface", False),
    (re.compile(r"^[ \t]*(?:export[ \t]+)?(?:declare[ \t]+)?type[ \t]+(?P<name>\w+)[^=\n]*=", re.M), "type", False),
    (re.compile(r"^[ \t]*(?:export[ \t]+)?(?:declare[ \t]+)?(?:const[ \t]+)?enum[ \t]+(?P<name>\w+)", re.M), "enum", False),
]

IMPORTS = {
    "go": [re.compile(r'^import[ \t]+(?:\w+[ \t]+)?"([^"]+)"', re.M)],
    "javascript": [re.compile(r"""^[ \t]*import\b[^'"\n]*?(?:from[ \t]*)?['"]([^'"]+)['"]""", re.M),
                   re.compile(r"""\brequire\(\s*['"]([^'"]+)['"]\s*\)""")],
    "java": [re.compile(r"^import[ \t]+(?:static[ \t]+)?([\w.*]+)[ \t]*;", re.M)],
    "rust": [re.compile(r"^[ \t]*(?:pub[ \t]+)?use[ \t]+([\w:]+)", re.M),
             re.compile(r"^[ \t]*(?:pub[ \t]+)?mod[ \t]+(\w+)[ \t]*;", re.M)],
}
IMPORTS["typescript"] = IMPORTS["javascript"]
_GO_IMPORT_BLOCK = re.compile(r"^import[ \t]*\(([^)]*)\)", re.M)
_CALL = re.compile(r"(?<![\w$])([A-Za-z_][\w]*)\s*(?:::<[^>]*>\s*)?\(")


def mask_code(text: str, language: str) -> str:
    """Blank out comment and string contents (keeping newlines and delimiters)."""
    out = list(text)
    n = len(text)
    i = 0
    backtick = language in ("go", "javascript", "typescript")

    def blank(a: int, b: int):
        for k in range(a, b):
            if out[k] != "\n":
                out[k] = " "

    while i < n:
        c = text[i]
        nxt = text[i + 1] if i + 1 < n else ""
        if c == "/" and nxt == "/":
            j = text.find("\n", i)
            j = n if j == -1 else j
            blank(i, j)
            i = j
        elif c == "/" and nxt == "*":
            j = text.find("*/", i + 2)
            j = n if j == -1 else j + 2
            blank(i, j)
            i = j
        elif language == "rust" and c == "r" and (nxt == '"' or nxt == "#") and (i == 0 or not (text[i - 1].isalnum() or text[i - 1] == "_")):
            m = re.match(r'r(#*)"', text[i:])
            if not m:
                i += 1
                continue
            close = '"' + m.group(1)
            j = text.find(close, i + len(m.group(0)))
            j = n if j == -1 else j + len(close)
            blank(i + 1, j - 1)
            i = j
        elif c == '"' or (c == "`" and backtick):
            j = i + 1
            while j < n and text[j] != c:
                if text[j] == "\\" and not (c == "`" and language == "go"):
                    j += 1
                elif text[j] == "\n" and c == '"':
                    break
                j += 1
            blank(i + 1, min(j, n))
            i = j + 1
        elif c == "'":
            if language == "rust":
                m = re.match(r"'(?:\\.[^'\n]{0,8}|[^\\'\n])'", text[i:])
                if not m:  # a lifetime such as 'a
                    i += 1
                    continue
                blank(i + 1, i + len(m.group(0)) - 1)
                i += len(m.group(0))
                continue
            j = i + 1
            while j < n and text[j] != "'" and text[j] != "\n":
                if text[j] == "\\":
                    j += 1
                j += 1
            blank(i + 1, min(j, n))
            i = j + 1
        else:
            i += 1
    return "".join(out)


def _find_body(masked: str, pos: int, language: str, kind: str) -> tuple[int | None, int]:
    """From the end of a declaration header, find its body.

    Returns (body_open_index or None, end_index_of_declaration).
    """
    n = len(masked)
    if kind == "arrow":
        j = pos
        while j < n and masked[j] in " \t":
            j += 1
        if j < n and masked[j] == "{":
            return j, j
        depth = 0
        while j < n:
            ch = masked[j]
            if ch in "([{":
                depth += 1
            elif ch in ")]}":
                depth -= 1
                if depth < 0:
                    return None, j
            elif depth == 0 and ch in ";\n":
                return None, j
            j += 1
        return None, n
    depth = 0
    i = pos
    limit = min(n, pos + 4000)
    while i < limit:
        ch = masked[i]
        if ch in "([":
            depth += 1
        elif ch in ")]":
            depth -= 1
        elif depth <= 0:
            if ch == ";":
                return None, i
            if ch == "\n" and (language == "go" and kind == "function" or kind == "type"):
                prev = masked[pos:i].rstrip()
                if not prev or prev[-1] not in "=|&,<(":
                    return None, i
            if ch == "{":
                # `interface{}` / `struct{}` inside a signature: skip when more header follows.
                j = i + 1
                while j < n and masked[j] in " \t":
                    j += 1
                if j < n and masked[j] == "}":
                    eol = masked.find("\n", j)
                    eol = n if eol == -1 else eol
                    if "{" in masked[j:eol]:
                        i = j + 1
                        continue
                return i, i
        i += 1
    eol = masked.find("\n", pos)
    return None, n if eol == -1 else eol


def _match_brace(masked: str, open_idx: int) -> int:
    depth = 0
    for i in range(open_idx, len(masked)):
        ch = masked[i]
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                return i
    return len(masked) - 1


def _leading_doc(lines: list[str], start_line: int) -> str:
    """Collect the comment block directly above a declaration (1-based line)."""
    doc: list[str] = []
    i = start_line - 2
    while i >= 0:
        s = lines[i].strip()
        if s.startswith(("#[", "@")) and not doc:
            i -= 1
            continue
        if s.startswith(("//", "/*", "*", "*/")):
            cleaned = re.sub(r"^(///?|/\*\*?|\*/|\*)\s?", "", s).rstrip("*/").strip()
            doc.append(cleaned)
            i -= 1
            continue
        break
    return "\n".join(reversed([d for d in doc if d])).strip()


def _brace_calls(masked_body: str, own_name: str) -> list[str]:
    seen: dict[str, None] = {}
    for m in _CALL.finditer(masked_body):
        name = m.group(1)
        if name in KEYWORDS:
            continue
        seen.setdefault(name)
    return list(seen)


def _go_receiver_type(recv: str) -> str:
    parts = recv.replace("*", " ").split()
    if not parts:
        return ""
    t = parts[-1]
    return t.split("[", 1)[0]


def _parse_brace(text: str, language: str) -> ParsedFile:
    masked = mask_code(text, language)
    lines = text.splitlines()
    line_starts = [0]
    for m in re.finditer("\n", text):
        line_starts.append(m.end())

    def line_of(offset: int) -> int:
        return bisect.bisect_right(line_starts, offset)

    raw = []  # (start_off, end_off, body_open, kind, name, extra, needs_container, header_end)
    taken: set[int] = set()
    for regex, kind, needs_container in DECLS[language]:
        for m in regex.finditer(masked):
            name = m.group("name")
            if name in KEYWORDS or name.lstrip("#") in KEYWORDS:
                continue
            start = m.start("name")
            line_start = masked.rfind("\n", 0, start) + 1
            if line_start in taken:
                continue
            gd = m.groupdict()
            k = kind
            if gd.get("kw"):
                k = {"struct": "struct", "interface": "interface", "enum": "enum", "trait": "trait",
                     "union": "struct", "class": "class", "record": "class",
                     "@interface": "interface"}[gd["kw"]]
            header_end = m.end() - 1 if masked[m.end() - 1] == "{" else m.end()
            body_open, end = _find_body(masked, header_end, language, kind)
            if body_open is not None:
                end = _match_brace(masked, body_open)
            taken.add(line_start)
            raw.append((line_start, end, body_open, k, name, gd, needs_container, m.end()))

    raw.sort(key=lambda r: (r[0], -r[1]))
    symbols: list[Symbol] = []
    stack: list[tuple[int, str, str]] = []  # (end_off, kind, qualname)
    for start, end, body_open, kind, name, gd, needs_container, header_end in raw:
        while stack and stack[-1][0] < start:
            stack.pop()
        parent = stack[-1] if stack else None
        if parent and parent[1] not in CONTAINER_KINDS:
            continue  # local helper inside a function body
        if needs_container and not parent:
            continue
        if language == "go" and gd.get("recv"):
            recv = _go_receiver_type(gd["recv"])
            kind, qual, parent_name = "method", f"{recv}.{name}", recv
        elif parent:
            parent_name = parent[2]
            qual = f"{parent_name}.{name}"
            if kind in ("function", "arrow"):
                kind = "method"
        else:
            qual, parent_name = name, ""
        if kind == "arrow":
            kind = "function"
        if kind == "impl":
            stack.append((end, "impl", name))
            continue  # impl blocks are containers, not indexed symbols themselves
        if kind in CONTAINER_KINDS:
            stack.append((end, kind, qual))
        sl, el = line_of(start), line_of(end)
        source = "\n".join(lines[sl - 1:el])
        header = text[start:body_open if body_open is not None else min(end, header_end + 200)]
        signature = " ".join(header.split())[:200]
        calls = _brace_calls(masked[body_open:end + 1], name) if body_open is not None and kind not in CONTAINER_KINDS else []
        symbols.append(Symbol(name, qual, kind, sl, el, source, signature,
                              _leading_doc(lines, sl), calls, parent_name))

    imports: list[str] = []
    for regex in IMPORTS.get(language, []):
        imports.extend(m.group(1) for m in regex.finditer(text))
    if language == "go":
        for block in _GO_IMPORT_BLOCK.finditer(masked):
            seg = text[block.start(1):block.end(1)]
            imports.extend(re.findall(r'"([^"]+)"', seg))
    module_doc = ""
    if language == "go":
        m = re.search(r"^package\s+\w+", text, re.M)
        if m:
            module_doc = _leading_doc(lines, line_of(m.start()))
    return ParsedFile(language, symbols, list(dict.fromkeys(imports)), module_doc)
