import type { GameEvent } from '../core/types.js';

/** Short Web Audio beeps for game events. Does nothing if audio is unavailable. */
export class AudioFx {
  private ctx: AudioContext | null = null;

  constructor(private enabled: boolean) {}

  private ensure(): AudioContext | null {
    if (!this.enabled) return null;
    if (!this.ctx) {
      const Ctor =
        (window as unknown as { AudioContext?: typeof AudioContext; webkitAudioContext?: typeof AudioContext })
          .AudioContext ??
        (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!Ctor) {
        this.enabled = false;
        return null;
      }
      this.ctx = new Ctor();
    }
    return this.ctx;
  }

  private beep(freq: number, dur: number, type: OscillatorType = 'square'): void {
    const ctx = this.ensure();
    if (!ctx) return;
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = type;
    osc.frequency.value = freq;
    gain.gain.value = 0.04;
    osc.connect(gain).connect(ctx.destination);
    const t = ctx.currentTime;
    osc.start(t);
    gain.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    osc.stop(t + dur);
  }

  handle(events: readonly GameEvent[]): void {
    if (!this.enabled) return;
    for (const e of events) {
      switch (e.type) {
        case 'shotFired':
          this.beep(880, 0.08);
          break;
        case 'alienKilled':
          this.beep(220, 0.12, 'sawtooth');
          break;
        case 'bombDropped':
          this.beep(140, 0.06, 'triangle');
          break;
        case 'playerHit':
          this.beep(90, 0.3, 'sawtooth');
          break;
        case 'ufoKilled':
          this.beep(660, 0.25, 'sine');
          break;
        case 'fleetDescended':
          this.beep(120, 0.05);
          break;
        case 'waveCleared':
          this.beep(990, 0.3, 'sine');
          break;
        case 'gameOver':
          this.beep(70, 0.6, 'sawtooth');
          break;
        default:
          break;
      }
    }
  }
}
