import { describe, it, expect } from "vitest";
import {
  clampSettings,
  DEFAULT_SETTINGS,
  SPEED_MIN,
  SPEED_MAX,
  FONT_MIN,
  FONT_MAX,
  LINE_SPACING_MIN,
  LINE_SPACING_MAX,
  MARGIN_MAX,
} from "../src/core/settings.js";

describe("clampSettings", () => {
  it("returns defaults for empty input", () => {
    expect(clampSettings()).toEqual(DEFAULT_SETTINGS);
    expect(clampSettings({})).toEqual(DEFAULT_SETTINGS);
  });

  it("merges partials over defaults", () => {
    const s = clampSettings({ fontSizePx: 80 });
    expect(s.fontSizePx).toBe(80);
    expect(s.speed).toBe(DEFAULT_SETTINGS.speed);
  });

  it("clamps speed into range", () => {
    expect(clampSettings({ speed: -100 }).speed).toBe(SPEED_MIN);
    expect(clampSettings({ speed: 99999 }).speed).toBe(SPEED_MAX);
    expect(clampSettings({ speed: 120 }).speed).toBe(120);
  });

  it("clamps font size into range", () => {
    expect(clampSettings({ fontSizePx: 1 }).fontSizePx).toBe(FONT_MIN);
    expect(clampSettings({ fontSizePx: 9999 }).fontSizePx).toBe(FONT_MAX);
  });

  it("clamps line spacing into range", () => {
    expect(clampSettings({ lineSpacing: 0 }).lineSpacing).toBe(LINE_SPACING_MIN);
    expect(clampSettings({ lineSpacing: 10 }).lineSpacing).toBe(LINE_SPACING_MAX);
  });

  it("clamps margins into range", () => {
    expect(clampSettings({ marginLeftPct: -5 }).marginLeftPct).toBe(0);
    expect(clampSettings({ marginRightPct: 200 }).marginRightPct).toBe(MARGIN_MAX);
  });

  it("rejects NaN / non-finite, falling back to defaults", () => {
    expect(clampSettings({ speed: NaN }).speed).toBe(DEFAULT_SETTINGS.speed);
    expect(clampSettings({ fontSizePx: Infinity }).fontSizePx).toBe(
      DEFAULT_SETTINGS.fontSizePx,
    );
  });

  it("falls back to default font for empty/blank font face", () => {
    expect(clampSettings({ fontFace: "" }).fontFace).toBe(DEFAULT_SETTINGS.fontFace);
    expect(clampSettings({ fontFace: "   " }).fontFace).toBe(DEFAULT_SETTINGS.fontFace);
    expect(clampSettings({ fontFace: "Georgia, serif" }).fontFace).toBe("Georgia, serif");
  });

  it("preserves boolean mirrored, ignores non-boolean", () => {
    expect(clampSettings({ mirrored: true }).mirrored).toBe(true);
    // @ts-expect-error testing runtime guard against bad input
    expect(clampSettings({ mirrored: "yes" }).mirrored).toBe(DEFAULT_SETTINGS.mirrored);
  });

  it("is idempotent", () => {
    const once = clampSettings({ speed: 500, fontSizePx: 5, lineSpacing: 9 });
    const twice = clampSettings(once);
    expect(twice).toEqual(once);
  });
});
