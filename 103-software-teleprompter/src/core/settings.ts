/** User settings, and clampSettings to keep each one in its allowed range. */

export interface Settings {
  /** Scroll speed in CSS pixels per second. */
  speed: number;
  /** CSS font-family stack. */
  fontFace: string;
  /** Font size in CSS pixels. */
  fontSizePx: number;
  /** CSS line-height multiplier. */
  lineSpacing: number;
  /** Left margin as a percentage of viewport width. */
  marginLeftPct: number;
  /** Right margin as a percentage of viewport width. */
  marginRightPct: number;
  /** Horizontal flip for beam-splitter rigs. */
  mirrored: boolean;
}

export const SPEED_MIN = 0;
export const SPEED_MAX = 400;
export const FONT_MIN = 16;
export const FONT_MAX = 160;
export const LINE_SPACING_MIN = 1.0;
export const LINE_SPACING_MAX = 3.0;
export const MARGIN_MIN = 0;
export const MARGIN_MAX = 40;

export const DEFAULT_SETTINGS: Settings = {
  speed: 80,
  fontFace:
    'system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
  fontSizePx: 64,
  lineSpacing: 1.4,
  marginLeftPct: 8,
  marginRightPct: 8,
  mirrored: false,
};

/** Clamp value into [min, max]; anything that isn't a finite number gives fallback. */
function clampNumber(
  value: unknown,
  min: number,
  max: number,
  fallback: number,
): number {
  const n = typeof value === "number" ? value : NaN;
  if (!Number.isFinite(n)) return fallback;
  if (n < min) return min;
  if (n > max) return max;
  return n;
}

/** Fill in defaults for missing fields and clamp the rest. */
export function clampSettings(partial: Partial<Settings> = {}): Settings {
  const base = DEFAULT_SETTINGS;

  const fontFace =
    typeof partial.fontFace === "string" && partial.fontFace.trim().length > 0
      ? partial.fontFace
      : base.fontFace;

  return {
    speed: clampNumber(partial.speed ?? base.speed, SPEED_MIN, SPEED_MAX, base.speed),
    fontFace,
    fontSizePx: clampNumber(
      partial.fontSizePx ?? base.fontSizePx,
      FONT_MIN,
      FONT_MAX,
      base.fontSizePx,
    ),
    lineSpacing: clampNumber(
      partial.lineSpacing ?? base.lineSpacing,
      LINE_SPACING_MIN,
      LINE_SPACING_MAX,
      base.lineSpacing,
    ),
    marginLeftPct: clampNumber(
      partial.marginLeftPct ?? base.marginLeftPct,
      MARGIN_MIN,
      MARGIN_MAX,
      base.marginLeftPct,
    ),
    marginRightPct: clampNumber(
      partial.marginRightPct ?? base.marginRightPct,
      MARGIN_MIN,
      MARGIN_MAX,
      base.marginRightPct,
    ),
    mirrored:
      typeof partial.mirrored === "boolean" ? partial.mirrored : base.mirrored,
  };
}
