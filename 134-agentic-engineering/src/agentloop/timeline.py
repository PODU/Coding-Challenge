"""Timeline rendering from transitions.jsonl."""

from __future__ import annotations

from pathlib import Path

from . import state as st


def render_table(run_dir: Path) -> str:
    entries = st.read_transitions(run_dir)
    if not entries:
        return "(no transitions recorded)"
    rows = [("TIME", "NODE", "FROM", "TO", "DUR", "DETAIL")]
    for e in entries:
        rows.append((e["ts"], e["node"], e["from"], e["to"],
                     f"{e['duration_s']:.1f}s",
                     e.get("detail", "")[:60]))
    widths = [max(len(r[i]) for r in rows) for i in range(5)]
    lines = []
    for r in rows:
        lines.append("  ".join(r[i].ljust(widths[i]) for i in range(5))
                     + "  " + r[5])
    total = sum(e["duration_s"] for e in entries)
    lines.append(f"\n{len(entries)} transitions, {total:.1f}s total node time")
    cand_root = run_dir / "candidates"
    if cand_root.is_dir():
        for d in sorted(cand_root.iterdir()):
            if d.is_dir() and st.read_transitions(d):
                lines.append(f"\n-- candidate {d.name} --")
                lines.append(render_table(d))
    return "\n".join(lines)


def render_mermaid(run_dir: Path) -> str:
    entries = st.read_transitions(run_dir)
    lines = ["gantt", "    title agentloop run", "    dateFormat X",
             "    axisFormat %S s", "    section nodes"]
    t = 0.0
    for e in entries:
        dur = max(e["duration_s"], 0.05)
        lines.append(f"    {e['node']} : {t:.2f}, {dur:.2f}s")
        t += dur
    return "\n".join(lines)
