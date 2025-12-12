/**
 * Scroll state for the teleprompter, with no DOM access.
 *
 * `position` is how many px have scrolled past the reading line. Each tick adds
 * speed * elapsed seconds, so the scroll rate doesn't depend on the frame rate.
 * Time only comes in through tick(now), which keeps tests deterministic.
 */

import type { Clock } from "./clock.js";
import { PlaybackState } from "./playback.js";
import { clampSettings, type Settings } from "./settings.js";

/** How close to maxPosition (in px) counts as being at the end. */
const END_EPSILON = 0.5;

export class ScrollEngine {
  private readonly clock: Clock;
  private _settings: Settings;
  private _state: PlaybackState = PlaybackState.Paused;
  private _position = 0;
  private _contentHeight = 0;
  private _viewportHeight = 0;

  /**
   * Time of the previous tick, or null before the first one. It is updated
   * while paused too, so resuming doesn't jump forward.
   */
  private _lastTick: number | null = null;

  constructor(clock: Clock, settings?: Settings) {
    this.clock = clock;
    this._settings = settings ? clampSettings(settings) : clampSettings();
  }

  get position(): number {
    return this._position;
  }

  get state(): PlaybackState {
    return this._state;
  }

  get settings(): Settings {
    return this._settings;
  }

  get contentHeight(): number {
    return this._contentHeight;
  }

  get viewportHeight(): number {
    return this._viewportHeight;
  }

  /** Largest scroll offset; 0 when everything fits in the viewport. */
  get maxPosition(): number {
    return Math.max(0, this._contentHeight - this._viewportHeight);
  }

  get atEnd(): boolean {
    return this._position >= this.maxPosition - END_EPSILON;
  }

  /** Progress from 0 to 1. Counts as 1 when there is nothing to scroll. */
  get progress(): number {
    const max = this.maxPosition;
    if (max <= 0) return 1;
    return Math.min(1, Math.max(0, this._position / max));
  }

  get isPlaying(): boolean {
    return this._state === PlaybackState.Playing;
  }

  /** Advance to time `now` (ms). Pauses automatically on reaching the end. */
  tick(now: number): void {
    const last = this._lastTick;
    this._lastTick = now;

    if (last === null) return;
    if (this._state !== PlaybackState.Playing) return;

    const dtSeconds = (now - last) / 1000;
    if (dtSeconds <= 0) return;

    this._position += this._settings.speed * dtSeconds;
    this.clampPosition();

    if (this.atEnd) {
      this._state = PlaybackState.Paused;
    }
  }

  /** Start scrolling, unless already playing or at the end. */
  play(): void {
    if (this._state === PlaybackState.Playing) return;
    if (this.atEnd) return;
    this._state = PlaybackState.Playing;
    // Otherwise the time spent paused would be counted on the next tick.
    this._lastTick = this.clock();
  }

  pause(): void {
    this._state = PlaybackState.Paused;
  }

  toggle(): void {
    if (this._state === PlaybackState.Playing) {
      this.pause();
    } else {
      this.play();
    }
  }

  /** Rewind to the top and pause. */
  reset(): void {
    this._position = 0;
    this._state = PlaybackState.Paused;
    this._lastTick = this.clock();
  }

  /** Jump to a position, clamped into range. Non-finite values are ignored. */
  seek(px: number): void {
    this._position = Number.isFinite(px) ? px : this._position;
    this.clampPosition();
  }

  setSpeed(pxPerSec: number): void {
    this.setSettings({ speed: pxPerSec });
  }

  /** Merge in new settings. Doesn't touch position or playback state. */
  setSettings(partial: Partial<Settings>): void {
    this._settings = clampSettings({ ...this._settings, ...partial });
  }

  setContentHeight(px: number): void {
    this._contentHeight = Number.isFinite(px) && px > 0 ? px : 0;
    this.clampPosition();
  }

  setViewportHeight(px: number): void {
    this._viewportHeight = Number.isFinite(px) && px > 0 ? px : 0;
    this.clampPosition();
  }

  private clampPosition(): void {
    const max = this.maxPosition;
    if (this._position < 0) this._position = 0;
    else if (this._position > max) this._position = max;
  }
}
