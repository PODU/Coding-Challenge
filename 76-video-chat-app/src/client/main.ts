/**
 * Browser side of the video chat.
 *
 * Creates or joins a room through the signalling API, grabs the camera and
 * microphone, then swaps SDP and ICE candidates with the other peer via the
 * server until a direct connection is up. Text chat goes over a data channel.
 *
 * Uses browser-only APIs, so it is tested by hand rather than in the suite.
 */

import type { JoinResponse, Role, SignalKind, SignalMessage } from '../shared/messages.js';

// Google's public STUN server is enough for most NATs. Symmetric NATs and
// strict firewalls need a TURN server as well.
const RTC_CONFIG: RTCConfiguration = {
  iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
  // iceServers: [{ urls: 'turn:turn.example.com:3478', username: 'u', credential: 'p' }],
};

type Status = 'idle' | 'joining' | 'connecting' | 'connected' | 'closed';

function el<T extends HTMLElement>(id: string): T {
  const node = document.getElementById(id);
  if (!node) throw new Error(`missing element #${id}`);
  return node as T;
}

const ui = {
  localVideo: el<HTMLVideoElement>('localVideo'),
  remoteVideo: el<HTMLVideoElement>('remoteVideo'),
  createBtn: el<HTMLButtonElement>('createBtn'),
  joinBtn: el<HTMLButtonElement>('joinBtn'),
  roomInput: el<HTMLInputElement>('roomInput'),
  muteBtn: el<HTMLButtonElement>('muteBtn'),
  cameraBtn: el<HTMLButtonElement>('cameraBtn'),
  hangupBtn: el<HTMLButtonElement>('hangupBtn'),
  status: el<HTMLSpanElement>('status'),
  roomLabel: el<HTMLSpanElement>('roomLabel'),
  chatLog: el<HTMLDivElement>('chatLog'),
  chatInput: el<HTMLInputElement>('chatInput'),
  chatSend: el<HTMLButtonElement>('chatSend'),
};

interface Session {
  roomId: string;
  peerId: string;
  role: Role;
  pc: RTCPeerConnection;
  localStream: MediaStream;
  dataChannel: RTCDataChannel | null;
  polling: boolean;
  /** ICE candidates that arrived before the remote description was set. */
  pendingCandidates: RTCIceCandidateInit[];
  remoteDescriptionSet: boolean;
}

let session: Session | null = null;

function setStatus(s: Status): void {
  ui.status.textContent = s;
  ui.status.dataset['state'] = s;
}

function appendChat(who: string, text: string): void {
  const line = document.createElement('div');
  line.className = 'chat-line';
  const label = document.createElement('span');
  label.className = 'chat-who';
  label.textContent = who + ': ';
  line.append(label, document.createTextNode(text));
  ui.chatLog.append(line);
  ui.chatLog.scrollTop = ui.chatLog.scrollHeight;
}

function notice(text: string): void {
  appendChat('system', text);
}

function setInCallUi(active: boolean): void {
  ui.createBtn.disabled = active;
  ui.joinBtn.disabled = active;
  ui.roomInput.disabled = active;
  ui.muteBtn.disabled = !active;
  ui.cameraBtn.disabled = !active;
  ui.hangupBtn.disabled = !active;
  ui.chatInput.disabled = !active;
  ui.chatSend.disabled = !active;
}

async function apiCreateRoom(): Promise<string> {
  const res = await fetch('/api/rooms', { method: 'POST' });
  if (!res.ok) throw new Error(`create room failed (${res.status})`);
  const body = (await res.json()) as { roomId: string };
  return body.roomId;
}

async function apiJoin(roomId: string): Promise<JoinResponse> {
  const res = await fetch(`/api/rooms/${encodeURIComponent(roomId)}/join`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({}),
  });
  if (res.status === 404) throw new Error('room not found');
  if (res.status === 409) throw new Error('room is full');
  if (!res.ok) throw new Error(`join failed (${res.status})`);
  return (await res.json()) as JoinResponse;
}

async function apiSignal(roomId: string, kind: SignalKind, from: string, payload: unknown): Promise<void> {
  await fetch(`/api/rooms/${encodeURIComponent(roomId)}/signal`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ from, kind, payload } satisfies SignalMessage),
  });
}

async function apiLeave(roomId: string, peerId: string): Promise<void> {
  try {
    await fetch(`/api/rooms/${encodeURIComponent(roomId)}/leave`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ peerId }),
      keepalive: true,
    });
  } catch {
    /* best effort on teardown */
  }
}

async function pollLoop(s: Session): Promise<void> {
  while (s.polling) {
    let messages: SignalMessage[] = [];
    try {
      const res = await fetch(
        `/api/rooms/${encodeURIComponent(s.roomId)}/poll?peerId=${encodeURIComponent(s.peerId)}`,
      );
      if (res.status === 404) {
        notice('room closed by server');
        hangup();
        return;
      }
      if (!res.ok) {
        await sleep(1000);
        continue;
      }
      messages = (await res.json()) as SignalMessage[];
    } catch {
      if (!s.polling) return;
      await sleep(1000);
      continue;
    }
    for (const msg of messages) {
      await handleSignalMessage(s, msg);
    }
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

async function handleSignalMessage(s: Session, msg: SignalMessage): Promise<void> {
  switch (msg.kind) {
    case 'offer': {
      await s.pc.setRemoteDescription(msg.payload as RTCSessionDescriptionInit);
      s.remoteDescriptionSet = true;
      await flushPendingCandidates(s);
      const answer = await s.pc.createAnswer();
      await s.pc.setLocalDescription(answer);
      await apiSignal(s.roomId, 'answer', s.peerId, answer);
      break;
    }
    case 'answer': {
      await s.pc.setRemoteDescription(msg.payload as RTCSessionDescriptionInit);
      s.remoteDescriptionSet = true;
      await flushPendingCandidates(s);
      break;
    }
    case 'ice': {
      const candidate = msg.payload as RTCIceCandidateInit;
      if (s.remoteDescriptionSet) {
        await s.pc.addIceCandidate(candidate).catch(() => undefined);
      } else {
        s.pendingCandidates.push(candidate);
      }
      break;
    }
    case 'bye': {
      notice('peer left the call');
      teardownRemote();
      setStatus('closed');
      break;
    }
  }
}

async function flushPendingCandidates(s: Session): Promise<void> {
  const queued = s.pendingCandidates;
  s.pendingCandidates = [];
  for (const c of queued) {
    await s.pc.addIceCandidate(c).catch(() => undefined);
  }
}

async function getLocalMedia(): Promise<MediaStream> {
  try {
    return await navigator.mediaDevices.getUserMedia({ audio: true, video: true });
  } catch (err) {
    const name = err instanceof DOMException ? err.name : 'error';
    if (name === 'NotAllowedError') throw new Error('camera/microphone permission denied');
    if (name === 'NotFoundError') throw new Error('no camera/microphone found');
    throw new Error('could not access camera/microphone');
  }
}

function buildPeerConnection(s: Session): void {
  const pc = s.pc;

  for (const track of s.localStream.getTracks()) {
    pc.addTrack(track, s.localStream);
  }

  pc.onicecandidate = (ev) => {
    if (ev.candidate) {
      void apiSignal(s.roomId, 'ice', s.peerId, ev.candidate.toJSON());
    }
  };

  pc.ontrack = (ev) => {
    const [stream] = ev.streams;
    if (stream) ui.remoteVideo.srcObject = stream;
  };

  pc.onconnectionstatechange = () => {
    if (pc.connectionState === 'connected') setStatus('connected');
    else if (pc.connectionState === 'failed' || pc.connectionState === 'disconnected') {
      setStatus('closed');
    }
  };

  // The initiator owns the data channel; the responder receives it.
  if (s.role === 'initiator') {
    wireDataChannel(s, pc.createDataChannel('chat'));
  } else {
    pc.ondatachannel = (ev) => wireDataChannel(s, ev.channel);
  }
}

function wireDataChannel(s: Session, channel: RTCDataChannel): void {
  s.dataChannel = channel;
  channel.onopen = () => notice('chat connected');
  channel.onclose = () => notice('chat closed');
  channel.onmessage = (ev) => appendChat('peer', String(ev.data));
}

async function startCall(roomId: string): Promise<void> {
  if (session) return;
  setStatus('joining');
  try {
    const join = await apiJoin(roomId);
    const localStream = await getLocalMedia();
    ui.localVideo.srcObject = localStream;

    const s: Session = {
      roomId,
      peerId: join.peerId,
      role: join.role,
      pc: new RTCPeerConnection(RTC_CONFIG),
      localStream,
      dataChannel: null,
      polling: true,
      pendingCandidates: [],
      remoteDescriptionSet: false,
    };
    session = s;
    buildPeerConnection(s);
    setInCallUi(true);
    ui.roomLabel.textContent = roomId;
    setStatus('connecting');

    void pollLoop(s);

    // The initiator kicks off negotiation with an offer; the responder waits.
    if (s.role === 'initiator') {
      const offer = await s.pc.createOffer();
      await s.pc.setLocalDescription(offer);
      await apiSignal(roomId, 'offer', s.peerId, offer);
      notice(`room "${roomId}" ready — share this id with the other person`);
    } else {
      notice(`joined room "${roomId}" — connecting`);
    }
  } catch (err) {
    setStatus('idle');
    setInCallUi(false);
    session = null;
    notice(`error: ${err instanceof Error ? err.message : String(err)}`);
  }
}

function teardownRemote(): void {
  const remote = ui.remoteVideo.srcObject;
  if (remote instanceof MediaStream) remote.getTracks().forEach((t) => t.stop());
  ui.remoteVideo.srcObject = null;
}

function hangup(): void {
  const s = session;
  if (!s) return;
  s.polling = false;
  void apiLeave(s.roomId, s.peerId);
  s.dataChannel?.close();
  s.pc.close();
  s.localStream.getTracks().forEach((t) => t.stop());
  ui.localVideo.srcObject = null;
  teardownRemote();
  session = null;
  setStatus('closed');
  setInCallUi(false);
  ui.roomLabel.textContent = '—';
}

function toggleAudio(): void {
  const s = session;
  if (!s) return;
  const track = s.localStream.getAudioTracks()[0];
  if (!track) return;
  track.enabled = !track.enabled;
  ui.muteBtn.textContent = track.enabled ? 'Mute' : 'Unmute';
}

function toggleVideo(): void {
  const s = session;
  if (!s) return;
  const track = s.localStream.getVideoTracks()[0];
  if (!track) return;
  track.enabled = !track.enabled;
  ui.cameraBtn.textContent = track.enabled ? 'Camera off' : 'Camera on';
}

function sendChat(): void {
  const s = session;
  const text = ui.chatInput.value.trim();
  if (!s || !s.dataChannel || s.dataChannel.readyState !== 'open' || text === '') return;
  s.dataChannel.send(text);
  appendChat('you', text);
  ui.chatInput.value = '';
}

ui.createBtn.addEventListener('click', () => {
  void (async () => {
    try {
      const roomId = await apiCreateRoom();
      ui.roomInput.value = roomId;
      history.replaceState(null, '', `?room=${encodeURIComponent(roomId)}`);
      await startCall(roomId);
    } catch (err) {
      notice(`error: ${err instanceof Error ? err.message : String(err)}`);
    }
  })();
});

ui.joinBtn.addEventListener('click', () => {
  const roomId = ui.roomInput.value.trim();
  if (roomId === '') {
    notice('enter a room id to join');
    return;
  }
  void startCall(roomId);
});

ui.muteBtn.addEventListener('click', toggleAudio);
ui.cameraBtn.addEventListener('click', toggleVideo);
ui.hangupBtn.addEventListener('click', hangup);
ui.chatSend.addEventListener('click', sendChat);
ui.chatInput.addEventListener('keydown', (ev) => {
  if (ev.key === 'Enter') sendChat();
});

window.addEventListener('beforeunload', () => {
  if (session) apiLeave(session.roomId, session.peerId);
});

// Prefill the room id from a shared `?room=` link.
const initialRoom = new URLSearchParams(location.search).get('room');
if (initialRoom) ui.roomInput.value = initialRoom;

setStatus('idle');
setInCallUi(false);
