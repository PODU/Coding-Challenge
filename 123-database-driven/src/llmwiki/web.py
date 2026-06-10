"""A small web chat UI. Needs the ``[web]`` extra (``pip install 'llmwiki[web]'``).

Citation markers in answers link to the cited page.
"""

from __future__ import annotations

from .wiki import Wiki

_HTML = """<!doctype html><html><head><meta charset="utf-8">
<title>LLM Wiki — {project}</title></head><body style="max-width:50rem;margin:2rem auto;font-family:system-ui">
<h1>LLM Wiki — {project}</h1>
<form onsubmit="ask(event)"><input id="q" style="width:80%" placeholder="Ask a question">
<button>Ask</button></form><div id="out"></div>
<script>
async function ask(e){{e.preventDefault();
 const q=document.getElementById('q').value;
 const r=await fetch('/api/{project}/ask',{{method:'POST',headers:{{'Content-Type':'application/json'}},
   body:JSON.stringify({{question:q}})}});
 const d=await r.json();
 let html='<p>'+d.answer+'</p><ul>';
 for(const c of d.citations){{html+='<li>^'+c.marker+' <a href="/page/{project}/'+c.page_id+'">'+c.page_id+'</a></li>';}}
 document.getElementById('out').innerHTML=html+'</ul>';}}
</script></body></html>"""


def create_app(wiki: Wiki):  # pragma: no cover - optional web seam
    from fastapi import FastAPI
    from fastapi.responses import HTMLResponse, JSONResponse

    app = FastAPI(title="LLM Wiki")

    @app.get("/", response_class=HTMLResponse)
    def index() -> str:
        projects = wiki.list_projects()
        return _HTML.format(project=projects[0] if projects else "default")

    @app.get("/ui/{project}", response_class=HTMLResponse)
    def ui(project: str) -> str:
        return _HTML.format(project=project)

    @app.post("/api/{project}/ask")
    def ask(project: str, payload: dict):
        ans = wiki.ask(project, payload["question"], k=payload.get("k", 6),
                       save=payload.get("save", False))
        return JSONResponse(ans.to_dict())

    @app.get("/api/{project}/pages")
    def pages(project: str, type: str | None = None):
        return [{"id": p.id, "type": p.type, "title": p.frontmatter.title}
                for p in wiki.browse(project, type=type)]

    @app.get("/api/{project}/page/{page_id:path}")
    def page(project: str, page_id: str):
        p = wiki.get_page(project, page_id)
        if p is None:
            return JSONResponse({"error": "not found"}, status_code=404)
        return {"id": p.id, "frontmatter": p.frontmatter.to_dict(), "body": p.body}

    return app


def run_server(wiki: Wiki, port: int = 8000) -> None:  # pragma: no cover - optional
    import uvicorn

    uvicorn.run(create_app(wiki), host="127.0.0.1", port=port)
