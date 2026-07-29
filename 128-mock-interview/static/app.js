// Mock Interview Coach frontend.
// Live mode streams the Anam persona into <video>; without ANAM_API_KEY the
// backend reports mock_video and we fall back to a text-chat interview.

const ANAM_SDK_URL = "https://esm.sh/@anam-ai/js-sdk@latest";

const state = {
  config: null,
  interview: null,      // {id, questions, setup}
  anamClient: null,
  transcript: [],       // [{role: "user"|"assistant", content}]
  skipped: [],
  currentQuestion: 0,
  timerHandle: null,
  secondsLeft: 0,
  ended: false,
};

const $ = (id) => document.getElementById(id);

const views = ["setup", "interview", "report", "history"];
function show(view) {
  for (const v of views) $(`view-${v}`).classList.toggle("hidden", v !== view);
  $("nav-new").classList.toggle("active", view !== "history");
  $("nav-history").classList.toggle("active", view === "history");
}

async function init() {
  state.config = await (await fetch("/api/config")).json();

  const comps = $("competencies");
  for (const c of state.config.competencies) {
    comps.insertAdjacentHTML(
      "beforeend",
      `<label class="check"><input type="checkbox" name="comp" value="${c}" checked> ${c}</label>`
    );
  }
  const styles = $("styles");
  state.config.styles.forEach((s, i) => {
    styles.insertAdjacentHTML(
      "beforeend",
      `<label class="check"><input type="radio" name="style" value="${s.id}" ${i === 0 ? "checked" : ""}>
        <strong>${s.name}</strong> — ${s.description}</label>`
    );
  });

  if (state.config.mock_video) {
    const b = $("mode-banner");
    b.textContent =
      "Running without ANAM_API_KEY — the interview will run as a text chat instead of live video.";
    b.classList.remove("hidden");
  }

  $("setup-form").addEventListener("submit", startInterview);
  $("btn-end").addEventListener("click", () => endInterview(true));
  $("btn-skip").addEventListener("click", skipQuestion);
  $("btn-again").addEventListener("click", () => location.reload());
  $("nav-new").addEventListener("click", () => location.reload());
  $("nav-history").addEventListener("click", showHistory);
  $("chat-form").addEventListener("submit", sendChatMessage);
}

async function startInterview(ev) {
  ev.preventDefault();
  const competencies = [...document.querySelectorAll('input[name="comp"]:checked')].map(
    (x) => x.value
  );
  const style = document.querySelector('input[name="style"]:checked').value;
  const resp = await fetch("/api/interviews", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      role: $("role").value,
      seniority: $("seniority").value,
      competencies,
      style,
      num_questions: parseInt($("num-questions").value, 10),
    }),
  });
  state.interview = await resp.json();
  state.transcript = [];
  state.skipped = [];
  state.currentQuestion = 0;
  state.ended = false;
  show("interview");

  const tokenResp = await fetch("/api/session-token", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ interview_id: state.interview.id }),
  });
  if (!tokenResp.ok) {
    const err = await tokenResp.json().catch(() => ({}));
    return status(`Could not start session: ${err.detail || tokenResp.status}`);
  }
  const token = await tokenResp.json();
  startTimer(token.max_session_seconds);

  if (token.mock) {
    await startTextMode();
  } else {
    await startVideoMode(token.session_token);
  }
}

async function startVideoMode(sessionToken) {
  status("Connecting to your interviewer…");
  try {
    const { createClient, AnamEvent } = await import(ANAM_SDK_URL);
    state.anamClient = createClient(sessionToken);
    state.anamClient.addListener(AnamEvent.MESSAGE_HISTORY_UPDATED, (messages) => {
      state.transcript = messages.map((m) => ({
        role: m.role === "user" ? "user" : "assistant",
        content: m.content,
      }));
      renderTranscript();
    });
    state.anamClient.addListener(AnamEvent.CONNECTION_CLOSED, () => {
      if (!state.ended) endInterview(false);
    });
    await state.anamClient.streamToVideoElement("interviewer-video");
    status("Connected — speak your answers out loud.");
  } catch (err) {
    status(`Video connection failed: ${err.message || err}`);
  }
}

async function startTextMode() {
  $("interviewer-video").classList.add("hidden");
  $("text-chat").classList.remove("hidden");
  await mockTurn(); // interviewer greets + asks question 1
}

async function sendChatMessage(ev) {
  ev.preventDefault();
  const input = $("chat-input");
  const text = input.value.trim();
  if (!text || state.ended) return;
  input.value = "";
  state.transcript.push({ role: "user", content: text });
  renderTranscript();
  await mockTurn();
}

async function mockTurn() {
  const resp = await fetch(`/api/interviews/${state.interview.id}/mock-chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ messages: state.transcript }),
  });
  const data = await resp.json();
  state.currentQuestion = data.question_index;
  state.transcript.push({ role: "assistant", content: data.reply });
  renderTranscript();
  if (data.done) await endInterview(false);
}

function skipQuestion() {
  const q = state.interview.questions[state.currentQuestion];
  if (q && !state.skipped.includes(q.id)) state.skipped.push(q.id);
  if (state.config.mock_video || !state.anamClient) {
    state.transcript.push({ role: "user", content: "I'd like to skip this question." });
    renderTranscript();
    mockTurn();
  } else if (typeof state.anamClient.sendUserMessage === "function") {
    state.anamClient.sendUserMessage("I'd like to skip this question, please move on.");
  } else {
    status('Say "I\'d like to skip this question" to your interviewer.');
  }
}

async function endInterview(userInitiated) {
  if (state.ended) return;
  state.ended = true;
  clearInterval(state.timerHandle);
  if (state.anamClient) {
    try { state.anamClient.stopStreaming(); } catch { /* already closed */ }
  }
  status("Interview finished. Saving transcript and generating feedback…");

  await fetch(`/api/interviews/${state.interview.id}/transcript`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      transcript: state.transcript,
      skipped_question_ids: state.skipped,
      ended_early: userInitiated,
    }),
  });

  const resp = await fetch(`/api/interviews/${state.interview.id}/feedback`, {
    method: "POST",
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    return status(`Feedback failed: ${err.detail || resp.status}`);
  }
  renderReport(await resp.json());
  show("report");
}

function startTimer(seconds) {
  state.secondsLeft = seconds;
  tick();
  state.timerHandle = setInterval(() => {
    state.secondsLeft -= 1;
    tick();
    if (state.secondsLeft <= 0) endInterview(false);
  }, 1000);
}
function tick() {
  const m = Math.floor(Math.max(0, state.secondsLeft) / 60);
  const s = Math.max(0, state.secondsLeft) % 60;
  $("timer").textContent = `${m}:${String(s).padStart(2, "0")}`;
  $("timer").classList.toggle("warn", state.secondsLeft <= 30);
}

function status(msg) {
  const el = $("interview-status");
  el.textContent = msg;
  el.classList.remove("hidden");
}

function turnHtml(t) {
  const who = t.role === "user" ? "You" : "Interviewer";
  return `<div class="turn ${t.role}"><span class="who">${who}</span>${escapeHtml(t.content)}</div>`;
}

function renderTranscript() {
  const html = state.transcript.map(turnHtml).join("");
  $("transcript-log").innerHTML = html;
  $("transcript-log").scrollTop = $("transcript-log").scrollHeight;
  const chat = $("chat-log");
  if (chat) {
    chat.innerHTML = html;
    chat.scrollTop = chat.scrollHeight;
  }
}

function starBadges(star) {
  return ["situation", "task", "action", "result"]
    .map(
      (k) =>
        `<span class="star ${star[k] ? "ok" : "miss"}">${k[0].toUpperCase()}${
          star[k] ? " ✓" : " ✗"
        }</span>`
    )
    .join("");
}

function renderReport(report, target) {
  const el = target || $("report-body");
  if (!report.answers.length) {
    el.innerHTML = `<p>${report.note || "No answered questions to score."}</p>`;
    return;
  }
  el.innerHTML = `
    <div class="report-summary">
      <div class="avg">Average score: <strong>${report.average_score} / 5</strong></div>
      <p><strong>Strengths:</strong> ${escapeHtml(report.overall_strengths)}</p>
    </div>
    ${report.answers
      .map(
        (a) => `
      <div class="answer-card">
        <h4>${escapeHtml(a.question)}</h4>
        <div class="score">Score: ${a.score}/5 &nbsp; ${starBadges(a.star)}</div>
        <p><strong>Improve:</strong> ${escapeHtml(a.improvement)}</p>
      </div>`
      )
      .join("")}`;
}

async function showHistory() {
  show("history");
  const [progress, sessions] = await Promise.all([
    (await fetch("/api/progress")).json(),
    (await fetch("/api/interviews")).json(),
  ]);

  const chart = $("progress-chart");
  chart.innerHTML = progress.sessions.length
    ? progress.sessions
        .map(
          (p) => `<div class="bar-row">
            <span class="bar-label">${new Date(p.created_at).toLocaleDateString()}</span>
            <div class="bar" style="width:${p.average_score * 18}%">${p.average_score}</div>
          </div>`
        )
        .join("")
    : "<p>No scored sessions yet.</p>";

  $("history-list").innerHTML = sessions.length
    ? sessions
        .map(
          (s) => `<div class="session-row" data-id="${s.id}">
            <span>${new Date(s.created_at).toLocaleString()}</span>
            <span>${escapeHtml(s.setup.seniority)} ${escapeHtml(s.setup.role)}</span>
            <span>${s.average_score != null ? s.average_score + "/5" : s.has_transcript ? "unscored" : "no transcript"}</span>
          </div>`
        )
        .join("")
    : "<p>No sessions yet.</p>";

  document.querySelectorAll(".session-row").forEach((row) =>
    row.addEventListener("click", () => showSessionDetail(row.dataset.id))
  );
  $("history-detail").innerHTML = "";
}

async function showSessionDetail(id) {
  const rec = await (await fetch(`/api/interviews/${id}`)).json();
  const detail = $("history-detail");
  detail.innerHTML = `
    <h3>Session ${new Date(rec.created_at).toLocaleString()}</h3>
    <h4>Transcript</h4>
    <div class="detail-transcript">${
      rec.transcript.length ? rec.transcript.map(turnHtml).join("") : "<p>No transcript.</p>"
    }</div>
    <h4>Feedback</h4>
    <div id="detail-report"></div>`;
  if (rec.feedback) renderReport(rec.feedback, $("detail-report"));
  else $("detail-report").innerHTML = "<p>No feedback generated.</p>";
  detail.scrollIntoView({ behavior: "smooth" });
}

function escapeHtml(s) {
  return String(s ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

init();
