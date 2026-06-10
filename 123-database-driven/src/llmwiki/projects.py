"""Projects: separate wikis that share nothing, in the store or on disk."""

from __future__ import annotations

import shutil
from pathlib import Path

from .interfaces import Store

WIKI_SUBDIRS = ("summaries", "entities", "topics", "answers")


class ProjectManager:
    def __init__(self, store: Store, projects_root: Path) -> None:
        self.store = store
        self.root = Path(projects_root)

    def project_dir(self, project: str) -> Path:
        return self.root / project

    def create(self, project: str) -> None:
        self.store.init_project(project)
        base = self.project_dir(project)
        for sub in WIKI_SUBDIRS:
            (base / "wiki" / sub).mkdir(parents=True, exist_ok=True)
        (base / "raw").mkdir(parents=True, exist_ok=True)

    def list(self) -> list[str]:
        return self.store.list_projects()

    def exists(self, project: str) -> bool:
        return project in self.store.list_projects()

    def delete(self, project: str) -> None:
        self.store.delete_project(project)
        base = self.project_dir(project)
        if base.exists():
            shutil.rmtree(base)
