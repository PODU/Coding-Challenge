import type { Segment } from "../core/geometry.js";

/**
 * Draws the wheel on a canvas. Angles are radians clockwise from the pointer at
 * 3 o'clock, which matches canvas angles since canvas y points down.
 */
export class WheelRenderer {
  private readonly ctx: CanvasRenderingContext2D;

  constructor(private readonly canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("2D canvas context unavailable");
    this.ctx = ctx;
  }

  /** Centre, radius and device pixel ratio for the current canvas size. */
  private get geom() {
    const dpr = (globalThis.devicePixelRatio as number) || 1;
    const w = this.canvas.width;
    const h = this.canvas.height;
    const cx = w / 2;
    const cy = h / 2;
    const radius = Math.min(w, h) / 2 - 14 * dpr;
    return { cx, cy, radius, dpr };
  }

  draw(segments: readonly Segment[], wheelAngle: number): void {
    const { ctx } = this;
    const { cx, cy, radius, dpr } = this.geom;
    ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

    if (segments.length === 0) {
      this.drawPlaceholder(cx, cy, radius, dpr);
      this.drawPointer(cx, cy, radius, dpr);
      return;
    }

    for (const seg of segments) {
      this.drawSegment(seg, segments.length, cx, cy, radius, wheelAngle, dpr);
    }

    // centre hub
    ctx.beginPath();
    ctx.arc(cx, cy, Math.max(10, radius * 0.06), 0, Math.PI * 2);
    ctx.fillStyle = "#1f2430";
    ctx.fill();
    ctx.lineWidth = 2 * dpr;
    ctx.strokeStyle = "#fff";
    ctx.stroke();

    this.drawPointer(cx, cy, radius, dpr);
  }

  private drawSegment(
    seg: Segment,
    count: number,
    cx: number,
    cy: number,
    radius: number,
    wheelAngle: number,
    dpr: number,
  ): void {
    const { ctx } = this;
    const a0 = seg.startAngle + wheelAngle;
    const a1 = seg.endAngle + wheelAngle;
    const arc = seg.endAngle - seg.startAngle;

    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.arc(cx, cy, radius, a0, a1, false);
    ctx.closePath();
    ctx.fillStyle = seg.color;
    ctx.fill();
    ctx.lineWidth = 1 * dpr;
    ctx.strokeStyle = "rgba(0,0,0,0.18)";
    ctx.stroke();

    // Skip labels on slices too thin to read.
    const minArc = 0.06; // about 3.4 degrees
    if (arc < minArc || count > 120) return;

    this.drawLabel(seg, cx, cy, radius, wheelAngle, arc, dpr);
  }

  private drawLabel(
    seg: Segment,
    cx: number,
    cy: number,
    radius: number,
    wheelAngle: number,
    arc: number,
    dpr: number,
  ): void {
    const { ctx } = this;
    const mid = seg.midAngle + wheelAngle;

    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate(mid);
    ctx.textAlign = "right";
    ctx.textBaseline = "middle";
    ctx.fillStyle = pickTextColor(seg.color);

    // Size the font to fit the slice's width at the label position.
    const maxRadial = radius * 0.78;
    const chord = 2 * (radius * 0.86) * Math.sin(Math.min(arc, Math.PI) / 2);
    let fontPx = Math.min(28 * dpr, Math.max(8 * dpr, chord * 0.62));

    ctx.font = `600 ${fontPx}px system-ui, sans-serif`;
    let text = seg.label;
    // Shrink the font, and truncate if that isn't enough.
    while (ctx.measureText(text).width > maxRadial && fontPx > 8 * dpr) {
      fontPx -= 1 * dpr;
      ctx.font = `600 ${fontPx}px system-ui, sans-serif`;
    }
    if (ctx.measureText(text).width > maxRadial) {
      text = ellipsize(ctx, text, maxRadial);
    }

    ctx.fillText(text, radius * 0.9, 0);
    ctx.restore();
  }

  private drawPointer(cx: number, cy: number, radius: number, dpr: number): void {
    const { ctx } = this;
    // At 3 o'clock, pointing in at the wheel.
    const x = cx + radius + 4 * dpr;
    const y = cy;
    const size = 16 * dpr;
    ctx.beginPath();
    ctx.moveTo(x, y - size);
    ctx.lineTo(x, y + size);
    ctx.lineTo(x - size * 1.2, y);
    ctx.closePath();
    ctx.fillStyle = "#e63946";
    ctx.fill();
    ctx.lineWidth = 2 * dpr;
    ctx.strokeStyle = "#fff";
    ctx.stroke();
  }

  private drawPlaceholder(cx: number, cy: number, radius: number, dpr: number): void {
    const { ctx } = this;
    ctx.beginPath();
    ctx.arc(cx, cy, radius, 0, Math.PI * 2);
    ctx.fillStyle = "#2a2f3a";
    ctx.fill();
    ctx.fillStyle = "#aeb6c4";
    ctx.font = `600 ${16 * dpr}px system-ui, sans-serif`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText("Add names to spin", cx, cy);
  }
}

function ellipsize(ctx: CanvasRenderingContext2D, text: string, maxWidth: number): string {
  const ell = "…";
  if (ctx.measureText(ell).width > maxWidth) return "";
  let t = text;
  while (t.length > 0 && ctx.measureText(t + ell).width > maxWidth) {
    t = t.slice(0, -1);
  }
  return t + ell;
}

/** Dark or light text depending on the lightness of an hsl() fill. */
function pickTextColor(color: string): string {
  const m = /hsl\(\s*[\d.]+\s*,\s*[\d.]+%\s*,\s*([\d.]+)%/.exec(color);
  if (m) {
    const l = parseFloat(m[1]);
    return l > 55 ? "#1a1a1a" : "#ffffff";
  }
  return "#ffffff";
}
