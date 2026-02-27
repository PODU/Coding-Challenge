/* Online Coding Platform — editor SPA (vanilla, no build step).
 *
 * Talks to the backend JSON API. Snippets live in localStorage. Degrades
 * gracefully when CodeMirror or localStorage are unavailable.
 */
(function () {
  'use strict';

  var SNIPPET_KEY = 'ocp.snippets';
  var els = {};
  var cm = null; // CodeMirror instance, or null when falling back to textarea
  var currentProblem = null;

  document.addEventListener('DOMContentLoaded', init);

  function init() {
    els.banner = document.getElementById('banner');
    els.editor = document.getElementById('editor');
    els.stdin = document.getElementById('stdin');
    els.output = document.getElementById('output');
    els.statusPill = document.getElementById('statusPill');
    els.cases = document.getElementById('cases');
    els.runBtn = document.getElementById('runBtn');
    els.submitBtn = document.getElementById('submitBtn');
    els.spinner = document.getElementById('spinner');
    els.languageSelect = document.getElementById('languageSelect');
    els.problemSelect = document.getElementById('problemSelect');
    els.statement = document.getElementById('statement');
    els.snippetName = document.getElementById('snippetName');
    els.saveSnippet = document.getElementById('saveSnippet');
    els.snippetList = document.getElementById('snippetList');

    setupEditor();
    loadLanguages();
    loadProblems();
    renderSnippets();

    els.runBtn.addEventListener('click', onRun);
    els.submitBtn.addEventListener('click', onSubmit);
    els.saveSnippet.addEventListener('click', onSaveSnippet);
    els.problemSelect.addEventListener('change', onProblemChange);
    els.languageSelect.addEventListener('change', onLanguageChange);
  }

  function setupEditor() {
    if (typeof CodeMirror === 'undefined') {
      banner('Rich editor failed to load — using a plain textarea.', 'warn');
      return; // textarea fallback
    }
    try {
      cm = CodeMirror.fromTextArea(els.editor, {
        lineNumbers: true,
        mode: 'python',
        theme: 'material-darker',
        indentUnit: 4,
        tabSize: 4,
      });
      cm.setSize('100%', '100%');
    } catch (e) {
      cm = null;
    }
  }

  function getSource() {
    return cm ? cm.getValue() : els.editor.value;
  }
  function setSource(text) {
    if (cm) cm.setValue(text);
    else els.editor.value = text;
  }
  function setMode(language) {
    if (cm) cm.setOption('mode', language === 'javascript' ? 'javascript' : 'python');
  }

  function api(path, options) {
    return fetch(path, options).then(function (res) {
      return res.json().then(function (body) {
        return { ok: res.ok, status: res.status, body: body };
      });
    });
  }

  function loadLanguages() {
    api('/api/languages')
      .then(function (r) {
        var langs = (r.body && r.body.languages) || [
          { id: 'python', label: 'Python' },
          { id: 'javascript', label: 'JavaScript' },
        ];
        els.languageSelect.innerHTML = '';
        langs.forEach(function (l) {
          var opt = document.createElement('option');
          opt.value = l.id;
          opt.textContent = l.label + (l.version ? ' — ' + l.version : '');
          els.languageSelect.appendChild(opt);
        });
      })
      .catch(function () {
        banner('Backend unreachable — start the server with `npm start`.', 'err');
      });
  }

  function loadProblems() {
    api('/api/problems')
      .then(function (r) {
        var problems = (r.body && r.body.problems) || [];
        problems.forEach(function (p) {
          var opt = document.createElement('option');
          opt.value = p.id;
          opt.textContent = 'Problem: ' + p.title;
          els.problemSelect.appendChild(opt);
        });
      })
      .catch(function () {
        /* problems are optional; ignore */
      });
  }

  function onLanguageChange() {
    setMode(els.languageSelect.value);
  }

  function onProblemChange() {
    var id = els.problemSelect.value;
    if (!id) {
      currentProblem = null;
      els.submitBtn.classList.add('hidden');
      els.statement.textContent = '';
      els.cases.innerHTML = '';
      return;
    }
    api('/api/problems/' + encodeURIComponent(id)).then(function (r) {
      if (!r.ok) {
        banner('Could not load problem.', 'err');
        return;
      }
      currentProblem = r.body;
      els.statement.textContent = r.body.statementMarkdown || '';
      els.submitBtn.classList.remove('hidden');
      var lang = els.languageSelect.value;
      var starter = (r.body.starterCode && r.body.starterCode[lang]) || '';
      if (starter) setSource(starter);
    });
  }

  function busy(on) {
    els.runBtn.disabled = on;
    els.submitBtn.disabled = on;
    els.spinner.classList.toggle('hidden', !on);
  }

  function onRun() {
    busy(true);
    els.cases.innerHTML = '';
    api('/api/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        language: els.languageSelect.value,
        source: getSource(),
        stdin: els.stdin.value,
      }),
    })
      .then(function (r) {
        renderRun(r.body, r.status);
      })
      .catch(function () {
        banner('Run failed — backend unreachable. Your code is preserved.', 'err');
      })
      .finally(function () {
        busy(false);
      });
  }

  function onSubmit() {
    if (!currentProblem) return;
    busy(true);
    els.output.textContent = '';
    api('/api/problems/' + encodeURIComponent(currentProblem.id) + '/submit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ language: els.languageSelect.value, source: getSource() }),
    })
      .then(function (r) {
        renderSubmit(r.body);
      })
      .catch(function () {
        banner('Submit failed — backend unreachable.', 'err');
      })
      .finally(function () {
        busy(false);
      });
  }

  function renderRun(result, httpStatus) {
    if (!result || httpStatus >= 400) {
      var msg = (result && result.error && result.error.message) || 'Request failed';
      els.statusPill.className = 'pill err';
      els.statusPill.textContent = 'error';
      els.output.textContent = msg;
      return;
    }
    setPill(result.status);
    var html = escapeHtml(result.stdout || '');
    if (result.error) {
      var line = result.error.line ? ' (line ' + result.error.line + ')' : '';
      html +=
        '<span class="stderr">' +
        escapeHtml('\n' + result.error.message + line + '\n') +
        escapeHtml(result.error.traceback || result.stderr || '') +
        '</span>';
    } else if (result.stderr) {
      html += '<span class="stderr">' + escapeHtml(result.stderr) + '</span>';
    }
    if (result.truncated) html += '\n[output truncated — limit reached]';
    html += '\n\n— ' + (result.timeMs != null ? result.timeMs + ' ms' : '');
    els.output.innerHTML = html;
  }

  function renderSubmit(result) {
    if (!result || !result.results) {
      banner('Submission rejected: ' + ((result && result.error && result.error.message) || ''), 'err');
      return;
    }
    setPill(result.passed ? 'ok' : 'error');
    els.statusPill.textContent =
      (result.passed ? 'PASSED ' : 'FAILED ') + result.passedCases + '/' + result.totalCases;
    els.output.textContent = '';
    els.cases.innerHTML = '';
    result.results.forEach(function (c) {
      var row = document.createElement('div');
      row.className = 'case ' + c.status;
      var label = c.name + (c.hidden ? ' (hidden)' : '');
      var diff = '';
      if (!c.hidden && c.status !== 'pass' && c.expected != null) {
        diff =
          '<span class="diff">expected ' +
          escapeHtml(JSON.stringify(c.expected)) +
          ' got ' +
          escapeHtml(JSON.stringify(c.actual || '')) +
          '</span>';
      }
      row.innerHTML =
        '<span class="verdict">' + c.status + '</span><span>' + escapeHtml(label) + '</span> ' + diff;
      els.cases.appendChild(row);
    });
  }

  function setPill(status) {
    var cls = status === 'ok' ? 'ok' : status === 'timeout' || status === 'output_limit' ? 'warn' : 'err';
    els.statusPill.className = 'pill ' + cls;
    els.statusPill.textContent = status;
  }

  function readSnippets() {
    try {
      var raw = localStorage.getItem(SNIPPET_KEY);
      return raw ? JSON.parse(raw) : {};
    } catch (e) {
      return {};
    }
  }

  function writeSnippets(map) {
    try {
      localStorage.setItem(SNIPPET_KEY, JSON.stringify(map));
      return true;
    } catch (e) {
      banner('Could not save snippet — storage full or disabled. Your code is still in the editor.', 'err');
      return false;
    }
  }

  function onSaveSnippet() {
    var name = (els.snippetName.value || '').trim();
    if (!name) {
      banner('Enter a snippet name first.', 'warn');
      return;
    }
    var map = readSnippets();
    map[name] = {
      name: name,
      language: els.languageSelect.value,
      source: getSource(),
      updatedAt: new Date().toISOString(),
    };
    if (writeSnippets(map)) {
      els.snippetName.value = '';
      renderSnippets();
    }
  }

  function renderSnippets() {
    var map = readSnippets();
    els.snippetList.innerHTML = '';
    Object.keys(map)
      .sort()
      .forEach(function (name) {
        var li = document.createElement('li');
        var label = document.createElement('span');
        label.className = 'name';
        label.textContent = name;
        label.title = 'Load ' + name;
        label.addEventListener('click', function () {
          var s = map[name];
          setSource(s.source);
          els.languageSelect.value = s.language;
          setMode(s.language);
        });
        var del = document.createElement('button');
        del.className = 'del';
        del.textContent = '×';
        del.title = 'Delete';
        del.addEventListener('click', function () {
          var m = readSnippets();
          delete m[name];
          writeSnippets(m);
          renderSnippets();
        });
        li.appendChild(label);
        li.appendChild(del);
        els.snippetList.appendChild(li);
      });
  }

  function banner(msg, kind) {
    els.banner.textContent = msg;
    els.banner.className = 'banner ' + (kind === 'warn' ? 'warn' : '');
    els.banner.classList.remove('hidden');
    setTimeout(function () {
      els.banner.classList.add('hidden');
    }, 6000);
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }
})();
