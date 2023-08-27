"use strict";

// Browser chat client: talks JSON over the WebSocket and shows messages,
// system notices and who's online.

const el = (id) => document.getElementById(id);

const joinView = el("join");
const chatView = el("chat");
const messagesEl = el("messages");
const membersEl = el("members");
const roomNameEl = el("room-name");
const meEl = el("me");

let ws = null;
let myName = "";
let myRoom = "";

function wsURL(user, room) {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const params = new URLSearchParams({ user, room });
  return `${proto}//${location.host}/ws?${params.toString()}`;
}

function fmtTime(ts) {
  if (!ts) return "";
  const d = new Date(ts * 1000);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function addMessage(from, text, ts) {
  const li = document.createElement("li");
  const who = document.createElement("span");
  who.className = "who";
  who.textContent = from;
  const body = document.createElement("span");
  body.textContent = text;
  const time = document.createElement("span");
  time.className = "ts";
  time.textContent = fmtTime(ts);
  li.append(who, body, time);
  appendAndScroll(li);
}

function addSystem(text, cls = "system") {
  const li = document.createElement("li");
  li.className = cls;
  li.textContent = text;
  appendAndScroll(li);
}

function appendAndScroll(li) {
  const atBottom = messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight < 40;
  messagesEl.appendChild(li);
  if (atBottom) messagesEl.scrollTop = messagesEl.scrollHeight;
}

function renderMembers(members) {
  membersEl.innerHTML = "";
  for (const m of members) {
    const li = document.createElement("li");
    li.textContent = m + (m === myName ? " (you)" : "");
    membersEl.appendChild(li);
  }
}

function handleFrame(frame) {
  switch (frame.type) {
    case "message":
      addMessage(frame.from, frame.text, frame.ts);
      break;
    case "system":
      addSystem(frame.text);
      break;
    case "presence":
      renderMembers(frame.members || []);
      break;
    case "join":
      // The server may have renamed us (e.g. "bob-2") to keep names unique.
      myName = frame.from;
      myRoom = frame.room;
      roomNameEl.textContent = myRoom;
      meEl.textContent = `· you are ${myName}`;
      break;
    case "error":
      addSystem(frame.text, "error");
      break;
  }
}

function connect(user, room) {
  ws = new WebSocket(wsURL(user, room));
  ws.onopen = () => {
    joinView.classList.add("hidden");
    chatView.classList.remove("hidden");
    el("text").focus();
  };
  ws.onmessage = (ev) => {
    try {
      handleFrame(JSON.parse(ev.data));
    } catch (_) {
      /* ignore malformed frames */
    }
  };
  ws.onclose = () => {
    addSystem("disconnected from server", "error");
  };
  ws.onerror = () => {
    showJoinError("could not connect to server");
  };
}

function send(obj) {
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(obj));
}

function showJoinError(msg) {
  const e = el("join-error");
  e.textContent = msg;
  e.hidden = false;
}

el("join-btn").addEventListener("click", () => {
  const user = el("join-user").value.trim();
  const room = el("join-room").value.trim() || "general";
  if (!user) {
    showJoinError("please enter a name");
    return;
  }
  el("join-error").hidden = true;
  connect(user, room);
});

el("join-user").addEventListener("keydown", (e) => {
  if (e.key === "Enter") el("join-btn").click();
});

el("composer").addEventListener("submit", (e) => {
  e.preventDefault();
  const input = el("text");
  const text = input.value.trim();
  if (!text) return;
  send({ type: "message", text });
  input.value = "";
});

el("switch-btn").addEventListener("click", () => {
  const room = el("switch-room").value.trim();
  if (!room || room === myRoom) return;
  messagesEl.innerHTML = "";
  send({ type: "join", room });
  el("switch-room").value = "";
});

el("leave-btn").addEventListener("click", () => {
  send({ type: "quit" });
  if (ws) ws.close();
  chatView.classList.add("hidden");
  joinView.classList.remove("hidden");
});
