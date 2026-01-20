"use strict";

const REFRESH_MS = 15000;
const listEl = document.getElementById("list");
const statusEl = document.getElementById("status");
const qEl = document.getElementById("q");
const minScoreEl = document.getElementById("minScore");
const typeEl = document.getElementById("type");

function buildURL() {
  const p = new URLSearchParams();
  p.set("limit", "50");
  if (qEl.value.trim()) p.set("q", qEl.value.trim());
  if (minScoreEl.value) p.set("min_score", minScoreEl.value);
  if (typeEl.value) p.set("type", typeEl.value);
  return "/stories?" + p.toString();
}

function escapeHTML(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

function render(stories) {
  if (!Array.isArray(stories) || stories.length === 0) {
    listEl.className = "empty";
    listEl.textContent = "No stories match the current filters.";
    return;
  }
  listEl.className = "";
  listEl.innerHTML = stories.map((s) => {
    const when = new Date(s.timestamp).toLocaleString();
    const title = escapeHTML(s.title || "(untitled)");
    const link = s.url
      ? `<a href="${escapeHTML(s.url)}" target="_blank" rel="noopener">${title}</a>`
      : title;
    const cat = s.category ? `<span class="tag">${escapeHTML(s.category)}</span>` : "";
    const typ = s.type ? `<span class="tag">${escapeHTML(s.type)}</span>` : "";
    return `<div class="story">
      <div class="score">${s.score | 0}</div>
      <div>
        <div>${link}${cat}${typ}</div>
        <div class="meta">by ${escapeHTML(s.author || "unknown")} &middot; ${when} &middot; ${escapeHTML(s.source || "")}</div>
      </div>
    </div>`;
  }).join("");
}

async function refresh() {
  try {
    statusEl.textContent = "refreshing...";
    const res = await fetch(buildURL());
    if (!res.ok) throw new Error("HTTP " + res.status);
    const data = await res.json();
    render(data);
    statusEl.textContent = "updated " + new Date().toLocaleTimeString();
  } catch (err) {
    statusEl.textContent = "error: " + err.message;
  }
}

let debounce;
[qEl, minScoreEl, typeEl].forEach((el) => el.addEventListener("input", () => {
  clearTimeout(debounce);
  debounce = setTimeout(refresh, 250);
}));

refresh();
setInterval(refresh, REFRESH_MS);
