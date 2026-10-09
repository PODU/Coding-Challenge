"""Language detection from file names (and shebangs for extensionless scripts)."""

from __future__ import annotations

from pathlib import PurePosixPath

EXTENSIONS = {
    ".py": "python", ".pyi": "python",
    ".go": "go",
    ".js": "javascript", ".jsx": "javascript", ".mjs": "javascript", ".cjs": "javascript",
    ".ts": "typescript", ".tsx": "typescript", ".mts": "typescript", ".cts": "typescript",
    ".java": "java",
    ".rs": "rust",
    ".c": "c", ".h": "c",
    ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".hpp": "cpp", ".hh": "cpp",
    ".cs": "csharp",
    ".rb": "ruby",
    ".php": "php",
    ".kt": "kotlin", ".swift": "swift", ".scala": "scala",
    ".sh": "shell", ".bash": "shell", ".zsh": "shell", ".ps1": "powershell",
    ".md": "markdown", ".rst": "rst", ".txt": "text",
    ".html": "html", ".htm": "html", ".css": "css", ".scss": "css",
    ".json": "json", ".yaml": "yaml", ".yml": "yaml", ".toml": "toml",
    ".ini": "ini", ".cfg": "ini", ".xml": "xml", ".sql": "sql",
}

FILENAMES = {
    "Makefile": "make", "Dockerfile": "docker", "go.mod": "gomod", "go.sum": "gosum",
}

SHEBANGS = {"python": "python", "node": "javascript", "bash": "shell", "sh": "shell"}

# Languages with a structural parser; everything else falls back to line windows.
PARSED = {"python", "go", "javascript", "typescript", "java", "rust"}


def detect(path: str, head: bytes = b"") -> str:
    p = PurePosixPath(path)
    if p.name in FILENAMES:
        return FILENAMES[p.name]
    lang = EXTENSIONS.get(p.suffix.lower())
    if lang:
        return lang
    if head.startswith(b"#!"):
        first = head.split(b"\n", 1)[0].decode("utf-8", "replace")
        for key, value in SHEBANGS.items():
            if key in first:
                return value
    return "text"
