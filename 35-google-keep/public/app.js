// Vanilla SPA client for the Keep clone. Talks to /api with the session cookie.

const COLORS = ['default','red','orange','yellow','green','teal','blue','purple','pink','gray'];

const $ = (id) => document.getElementById(id);
const state = { view: 'notes', label: null, q: '', notes: [], user: null, editing: null };

async function api(path, opts = {}) {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    ...opts,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const msg = data?.error?.message || `Request failed (${res.status})`;
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return data;
}

function toast(msg, isError = false) {
  const t = $('toast');
  t.textContent = msg;
  t.className = 'toast' + (isError ? ' error' : '');
  t.hidden = false;
  clearTimeout(toast._t);
  toast._t = setTimeout(() => (t.hidden = true), 3000);
}

async function checkAuth() {
  try {
    const { user } = await api('/api/auth/me');
    state.user = user;
    showApp();
  } catch {
    showLogin();
  }
}

function showLogin() {
  $('login').hidden = false;
  for (const id of ['topbar', 'sidebar', 'app']) $(id).hidden = true;
}

function showApp() {
  $('login').hidden = true;
  for (const id of ['topbar', 'sidebar', 'app']) $(id).hidden = false;
  $('user-name').textContent = state.user.displayName || state.user.email || '';
  loadLabels();
  loadNotes();
}

$('local-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const username = $('local-username').value.trim();
  if (!username) return;
  try {
    await api('/api/auth/local', { method: 'POST', body: { username } });
    await checkAuth();
  } catch (err) {
    $('login-error').textContent = err.message;
  }
});

$('logout').addEventListener('click', async () => {
  await api('/api/auth/logout', { method: 'POST' });
  state.user = null;
  showLogin();
});

async function loadNotes() {
  const params = new URLSearchParams();
  if (state.view === 'archived') params.set('archived', '1');
  if (state.view === 'trashed') params.set('trashed', '1');
  if (state.q) params.set('q', state.q);
  if (state.label) params.set('label', state.label);
  try {
    const { notes } = await api('/api/notes?' + params.toString());
    state.notes = notes;
    render();
  } catch (err) {
    toast('Could not load notes: ' + err.message, true);
  }
}

async function loadLabels() {
  try {
    const { labels } = await api('/api/labels');
    const box = $('nav-labels');
    box.innerHTML = '';
    for (const l of labels) {
      const b = document.createElement('button');
      b.className = 'nav-item' + (state.label === l.name ? ' active' : '');
      b.textContent = '🏷️ ' + l.name;
      b.onclick = () => { state.label = state.label === l.name ? null : l.name; state.view = 'notes'; setActiveNav(); loadNotes(); };
      box.appendChild(b);
    }
  } catch { /* ignore */ }
}

function render() {
  const grid = $('grid');
  grid.innerHTML = '';
  $('composer').style.display = state.view === 'notes' && !state.label ? 'block' : 'none';
  $('empty').hidden = state.notes.length > 0;
  for (const n of state.notes) grid.appendChild(noteCard(n));
}

function noteCard(n) {
  const el = document.createElement('div');
  el.className = `note color-${n.color}` + (n.pinned ? ' pinned' : '');
  el.onclick = (e) => { if (!e.target.closest('button')) openEditor(n); };

  const pin = iconBtn(n.pinned ? '📌' : '📍', 'Pin', async (ev) => {
    ev.stopPropagation();
    await updateNote(n.id, { pinned: !n.pinned });
  });
  pin.className = 'icon-btn pin-btn';
  el.appendChild(pin);

  if (n.title) { const h = document.createElement('h3'); h.textContent = n.title; el.appendChild(h); }
  if (n.content) { const p = document.createElement('p'); p.textContent = n.content; el.appendChild(p); }

  if (n.labels.length) {
    const wrap = document.createElement('div');
    wrap.className = 'labels';
    for (const name of n.labels) {
      const c = document.createElement('span'); c.className = 'label-chip'; c.textContent = name; wrap.appendChild(c);
    }
    el.appendChild(wrap);
  }

  const row = document.createElement('div');
  row.className = 'row';
  if (state.view === 'trashed') {
    row.appendChild(iconBtn('♻️', 'Restore', () => restoreNote(n.id)));
    row.appendChild(iconBtn('❌', 'Delete forever', () => purgeNote(n.id)));
  } else {
    row.appendChild(iconBtn(n.archived ? '📤' : '📥', n.archived ? 'Unarchive' : 'Archive',
      () => updateNote(n.id, { archived: !n.archived })));
    row.appendChild(iconBtn('🗑️', 'Delete', () => deleteNote(n.id)));
  }
  el.appendChild(row);
  return el;
}

function iconBtn(label, title, onClick) {
  const b = document.createElement('button');
  b.className = 'icon-btn';
  b.textContent = label;
  b.title = title;
  b.onclick = (e) => { e.stopPropagation(); onClick(e); };
  return b;
}

async function createNote() {
  const title = $('c-title').value.trim();
  const content = $('c-content').value.trim();
  if (!title && !content) return;
  const labels = $('c-labels').value.split(',').map((s) => s.trim()).filter(Boolean);
  try {
    await api('/api/notes', { method: 'POST', body: { title, content, labels } });
    $('c-title').value = ''; $('c-content').value = ''; $('c-labels').value = '';
    await loadLabels();
    await loadNotes();
  } catch (err) {
    toast('Failed to add note: ' + err.message, true);
  }
}

async function updateNote(id, patch) {
  try {
    await api('/api/notes/' + id, { method: 'PATCH', body: patch });
    await loadLabels();
    await loadNotes();
  } catch (err) { toast('Update failed: ' + err.message, true); }
}

async function deleteNote(id) {
  try { await api('/api/notes/' + id, { method: 'DELETE' }); toast('Note moved to trash'); await loadNotes(); }
  catch (err) { toast('Delete failed: ' + err.message, true); }
}
async function restoreNote(id) {
  try { await api('/api/notes/' + id + '/restore', { method: 'POST' }); await loadNotes(); }
  catch (err) { toast('Restore failed: ' + err.message, true); }
}
async function purgeNote(id) {
  try { await api('/api/notes/' + id + '?purge=1', { method: 'DELETE' }); toast('Deleted forever'); await loadNotes(); }
  catch (err) { toast('Delete failed: ' + err.message, true); }
}

$('c-save').addEventListener('click', createNote);

function openEditor(n) {
  state.editing = { ...n };
  $('m-title').value = n.title;
  $('m-content').value = n.content;
  renderModalLabels();
  renderPalette();
  $('modal').hidden = false;
}

function renderModalLabels() {
  const box = $('m-labels');
  box.innerHTML = '';
  for (const name of state.editing.labels) {
    const chip = document.createElement('span');
    chip.className = 'label-chip';
    chip.textContent = name + ' ✕';
    chip.style.cursor = 'pointer';
    chip.onclick = () => { state.editing.labels = state.editing.labels.filter((l) => l !== name); renderModalLabels(); };
    box.appendChild(chip);
  }
}

function renderPalette() {
  const box = $('m-palette');
  box.innerHTML = '';
  for (const c of COLORS) {
    const s = document.createElement('div');
    s.className = 'swatch color-' + c + (state.editing.color === c ? ' sel' : '');
    s.title = c;
    s.onclick = () => { state.editing.color = c; renderPalette(); };
    box.appendChild(s);
  }
}

$('m-labels-input').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    e.preventDefault();
    const v = e.target.value.trim();
    if (v && !state.editing.labels.includes(v)) state.editing.labels.push(v);
    e.target.value = '';
    renderModalLabels();
  }
});

$('m-close').addEventListener('click', async () => {
  const e = state.editing;
  $('modal').hidden = true;
  try {
    await api('/api/notes/' + e.id, {
      method: 'PATCH',
      body: { title: $('m-title').value, content: $('m-content').value, color: e.color, labels: e.labels },
    });
    await loadLabels();
    await loadNotes();
  } catch (err) { toast('Save failed: ' + err.message, true); }
});

function setActiveNav() {
  document.querySelectorAll('#sidebar .nav-item[data-view]').forEach((b) => {
    b.classList.toggle('active', b.dataset.view === state.view && !state.label);
  });
  loadLabels();
}

document.querySelectorAll('.nav-item[data-view]').forEach((b) => {
  b.addEventListener('click', () => {
    state.view = b.dataset.view; state.label = null; setActiveNav(); loadNotes();
  });
});

let searchTimer;
$('search').addEventListener('input', (e) => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => { state.q = e.target.value.trim(); loadNotes(); }, 250);
});

checkAuth();
