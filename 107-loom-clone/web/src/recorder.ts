// The recording page. There are no automated tests for this file since it needs
// real media devices and permission prompts.
import { ResumableUploader } from './uploader';

function $<T extends HTMLElement>(id: string): T {
  const el = document.getElementById(id);
  if (!el) {
    throw new Error(`Missing element #${id}`);
  }
  return el as T;
}

const PREFERRED_MIME_TYPES = [
  'video/webm;codecs=vp9,opus',
  'video/webm;codecs=vp8,opus',
  'video/webm',
  'video/mp4',
];

function pickMimeType(): string {
  for (const type of PREFERRED_MIME_TYPES) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(type)) {
      return type;
    }
  }
  return '';
}

function fileExtFor(mime: string): string {
  return mime.includes('mp4') ? 'mp4' : 'webm';
}

function timestampName(ext: string): string {
  const d = new Date();
  const p = (n: number): string => String(n).padStart(2, '0');
  return `loom-${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(
    d.getMinutes(),
  )}${p(d.getSeconds())}.${ext}`;
}

class RecorderController {
  private cameraSelect = $<HTMLSelectElement>('camera');
  private micSelect = $<HTMLSelectElement>('mic');
  private enableCamera = $<HTMLInputElement>('enableCamera');
  private enableMic = $<HTMLInputElement>('enableMic');
  private enableScreen = $<HTMLInputElement>('enableScreen');
  private preview = $<HTMLVideoElement>('preview');
  private startBtn = $<HTMLButtonElement>('start');
  private stopBtn = $<HTMLButtonElement>('stop');
  private titleInput = $<HTMLInputElement>('title');
  private statusEl = $<HTMLDivElement>('status');
  private resultEl = $<HTMLDivElement>('result');

  private camStream: MediaStream | null = null;
  private screenStream: MediaStream | null = null;
  private recorder: MediaRecorder | null = null;
  private composite: { canvas: HTMLCanvasElement; raf: number } | null = null;
  private uploader: ResumableUploader | null = null;
  private startedAt = 0;

  async init(): Promise<void> {
    this.startBtn.addEventListener('click', () => void this.start());
    this.stopBtn.addEventListener('click', () => void this.stop());
    this.enableCamera.addEventListener('change', () => this.applyToggles());
    this.enableMic.addEventListener('change', () => this.applyToggles());
    this.cameraSelect.addEventListener('change', () => void this.refreshPreview());
    this.micSelect.addEventListener('change', () => void this.refreshPreview());
    this.stopBtn.disabled = true;

    try {
      // Device labels stay empty until the user has granted access once.
      const probe = await navigator.mediaDevices.getUserMedia({ audio: true, video: true });
      probe.getTracks().forEach((t) => t.stop());
    } catch {
      this.setStatus('Camera/mic permission not granted yet — labels may be hidden.', true);
    }
    await this.enumerate();
    await this.refreshPreview();
  }

  private setStatus(msg: string, isError = false): void {
    this.statusEl.textContent = msg;
    this.statusEl.className = isError ? 'status error' : 'status';
  }

  private async enumerate(): Promise<void> {
    const devices = await navigator.mediaDevices.enumerateDevices();
    fillSelect(this.cameraSelect, devices.filter((d) => d.kind === 'videoinput'), 'Camera');
    fillSelect(this.micSelect, devices.filter((d) => d.kind === 'audioinput'), 'Microphone');
  }

  private async refreshPreview(): Promise<void> {
    this.stopCamStream();
    if (!this.enableCamera.checked && !this.enableMic.checked) {
      this.preview.srcObject = null;
      return;
    }
    try {
      const constraints: MediaStreamConstraints = {
        video: this.enableCamera.checked ? deviceConstraint(this.cameraSelect.value) : false,
        audio: this.enableMic.checked ? deviceConstraint(this.micSelect.value) : false,
      };
      this.camStream = await navigator.mediaDevices.getUserMedia(constraints);
      this.preview.srcObject = this.camStream;
      this.preview.muted = true;
      await this.preview.play().catch(() => undefined);
      this.applyToggles();
    } catch (err) {
      this.handleMediaError(err);
    }
  }

  private applyToggles(): void {
    if (!this.camStream) {
      void this.refreshPreview();
      return;
    }
    this.camStream.getVideoTracks().forEach((t) => (t.enabled = this.enableCamera.checked));
    this.camStream.getAudioTracks().forEach((t) => (t.enabled = this.enableMic.checked));
  }

  private async buildRecordingStream(): Promise<MediaStream> {
    const wantScreen = this.enableScreen.checked;
    const wantCamera = this.enableCamera.checked;

    if (wantScreen) {
      this.screenStream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true });
      // The browser's own "Stop sharing" button ends this track.
      const [screenTrack] = this.screenStream.getVideoTracks();
      screenTrack?.addEventListener('ended', () => void this.stop());
    }

    const audioTracks: MediaStreamTrack[] = [];
    if (this.camStream && this.enableMic.checked) {
      audioTracks.push(...this.camStream.getAudioTracks());
    }
    if (this.screenStream) {
      audioTracks.push(...this.screenStream.getAudioTracks());
    }

    // Screen and camera together: draw the camera picture-in-picture on a canvas.
    if (wantScreen && wantCamera && this.camStream) {
      return this.compositeStream(this.screenStream as MediaStream, this.camStream, audioTracks);
    }
    if (wantScreen && this.screenStream) {
      return new MediaStream([...this.screenStream.getVideoTracks(), ...audioTracks]);
    }
    if (this.camStream) {
      const video = wantCamera ? this.camStream.getVideoTracks() : [];
      return new MediaStream([...video, ...audioTracks]);
    }
    throw new Error('Nothing to record — enable screen and/or camera.');
  }

  private compositeStream(
    screen: MediaStream,
    camera: MediaStream,
    audioTracks: MediaStreamTrack[],
  ): MediaStream {
    const screenVideo = document.createElement('video');
    screenVideo.srcObject = new MediaStream(screen.getVideoTracks());
    screenVideo.muted = true;
    void screenVideo.play();
    const camVideo = document.createElement('video');
    camVideo.srcObject = new MediaStream(camera.getVideoTracks());
    camVideo.muted = true;
    void camVideo.play();

    const canvas = document.createElement('canvas');
    canvas.width = 1280;
    canvas.height = 720;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      throw new Error('Canvas 2D context unavailable');
    }

    const draw = (): void => {
      ctx.drawImage(screenVideo, 0, 0, canvas.width, canvas.height);
      if (this.enableCamera.checked && camVideo.videoWidth > 0) {
        const pipW = canvas.width * 0.22;
        const pipH = (pipW * camVideo.videoHeight) / camVideo.videoWidth;
        ctx.drawImage(camVideo, canvas.width - pipW - 16, canvas.height - pipH - 16, pipW, pipH);
      }
      const raf = requestAnimationFrame(draw);
      if (this.composite) {
        this.composite.raf = raf;
      }
    };
    const raf = requestAnimationFrame(draw);
    this.composite = { canvas, raf };

    const canvasStream = canvas.captureStream(30);
    return new MediaStream([...canvasStream.getVideoTracks(), ...audioTracks]);
  }

  private async start(): Promise<void> {
    try {
      this.resultEl.textContent = '';
      const mimeType = pickMimeType();
      if (mimeType === '') {
        this.setStatus('No supported recording codec in this browser.', true);
        return;
      }
      const stream = await this.buildRecordingStream();
      const recorder = new MediaRecorder(stream, { mimeType });
      this.recorder = recorder;

      this.uploader = new ResumableUploader({
        filename: timestampName(fileExtFor(mimeType)),
        mimeType,
      });
      await this.uploader.start();

      recorder.addEventListener('dataavailable', (ev: BlobEvent) => {
        if (ev.data && ev.data.size > 0) {
          this.uploader?.addChunk(ev.data);
        }
      });
      recorder.addEventListener('stop', () => void this.finalize());

      this.startedAt = Date.now();
      recorder.start(3000); // a chunk every 3s, uploaded while still recording
      this.startBtn.disabled = true;
      this.stopBtn.disabled = false;
      this.setStatus('Recording…');
    } catch (err) {
      this.handleMediaError(err);
    }
  }

  private stop(): void {
    if (this.recorder && this.recorder.state !== 'inactive') {
      this.recorder.stop();
    }
    this.stopBtn.disabled = true;
    this.setStatus('Finishing upload…');
  }

  private async finalize(): Promise<void> {
    this.stopTracks();
    const durationMs = Date.now() - this.startedAt;
    try {
      if (!this.uploader) {
        return;
      }
      const title = this.titleInput.value.trim();
      const rec = await this.uploader.finish({
        ...(title ? { title } : {}),
        durationMs,
      });
      this.setStatus('Done.');
      const link = `${location.origin}/v/${rec.shareId}`;
      this.resultEl.innerHTML = `Share link: <a href="${link}">${link}</a>`;
    } catch (err) {
      this.setStatus(`Upload failed: ${(err as Error).message}`, true);
    } finally {
      this.startBtn.disabled = false;
    }
  }

  private handleMediaError(err: unknown): void {
    const name = err instanceof DOMException ? err.name : '';
    if (name === 'NotAllowedError') {
      this.setStatus('Permission denied. Allow camera/microphone/screen access and retry.', true);
    } else if (name === 'NotFoundError') {
      this.setStatus('No matching device found.', true);
    } else {
      this.setStatus(`Error: ${(err as Error).message ?? String(err)}`, true);
    }
    this.startBtn.disabled = false;
    this.stopBtn.disabled = true;
  }

  private stopCamStream(): void {
    this.camStream?.getTracks().forEach((t) => t.stop());
    this.camStream = null;
  }

  private stopTracks(): void {
    if (this.composite) {
      cancelAnimationFrame(this.composite.raf);
      this.composite = null;
    }
    this.screenStream?.getTracks().forEach((t) => t.stop());
    this.screenStream = null;
    this.stopCamStream();
    this.preview.srcObject = null;
  }
}

function deviceConstraint(deviceId: string): MediaTrackConstraints | boolean {
  return deviceId ? { deviceId: { exact: deviceId } } : true;
}

function fillSelect(select: HTMLSelectElement, devices: MediaDeviceInfo[], label: string): void {
  select.innerHTML = '';
  devices.forEach((d, i) => {
    const opt = document.createElement('option');
    opt.value = d.deviceId;
    opt.textContent = d.label || `${label} ${i + 1}`;
    select.appendChild(opt);
  });
}

void new RecorderController().init();
