/**
 * GameState → canvas (browser-only, impure, read-only over state).
 *
 * Classic vector look: white outlines on black. The renderer never mutates the
 * game state; it only draws it.
 */

import { SHIP_RADIUS } from "../core/constants.js";
import { fromAngle } from "../core/vec2.js";
import type { Asteroid, GameState, Ship } from "../types.js";

export class CanvasRenderer {
  private readonly ctx: CanvasRenderingContext2D;

  constructor(ctx: CanvasRenderingContext2D) {
    this.ctx = ctx;
  }

  draw(state: GameState): void {
    const { ctx } = this;
    const { width, height } = state;

    ctx.fillStyle = "#000";
    ctx.fillRect(0, 0, width, height);
    ctx.lineWidth = 1.5;
    ctx.strokeStyle = "#fff";
    ctx.fillStyle = "#fff";

    for (const a of state.asteroids) this.drawAsteroidWrapped(a, width, height);
    for (const b of state.bullets) this.drawDotWrapped(b.pos.x, b.pos.y, 2, width, height);

    if (state.ship.alive && this.shipVisible(state)) {
      this.drawShipWrapped(state.ship, width, height);
    }

    this.drawHud(state);

    if (state.phase === "attract") {
      this.drawCenterText("ASTEROIDS", "Press ENTER to start", width, height);
    } else if (state.phase === "game-over") {
      this.drawCenterText(
        "GAME OVER",
        `Score ${state.score}  —  Press ENTER to play again`,
        width,
        height,
      );
    }
  }

  /** Blink the ship while invulnerable so the immunity is visible. */
  private shipVisible(state: GameState): boolean {
    if (state.ship.invulnerable <= 0) return true;
    return Math.floor(state.ship.invulnerable * 10) % 2 === 0;
  }

  /** Draw `fn` at the wrapped offsets so objects straddling a seam show twice. */
  private wrapped(
    x: number,
    y: number,
    r: number,
    w: number,
    h: number,
    fn: (ox: number, oy: number) => void,
  ): void {
    const xs = [x];
    const ys = [y];
    if (x < r) xs.push(x + w);
    else if (x > w - r) xs.push(x - w);
    if (y < r) ys.push(y + h);
    else if (y > h - r) ys.push(y - h);
    for (const ox of xs) for (const oy of ys) fn(ox, oy);
  }

  private drawShipWrapped(ship: Ship, w: number, h: number): void {
    this.wrapped(ship.pos.x, ship.pos.y, SHIP_RADIUS * 2, w, h, (ox, oy) =>
      this.drawShip(ship, ox, oy),
    );
  }

  private drawShip(ship: Ship, x: number, y: number): void {
    const { ctx } = this;
    const a = ship.angle;
    const nose = fromAngle(a);
    const left = fromAngle(a + (Math.PI * 3) / 4);
    const right = fromAngle(a - (Math.PI * 3) / 4);
    ctx.beginPath();
    ctx.moveTo(x + nose.x * SHIP_RADIUS, y + nose.y * SHIP_RADIUS);
    ctx.lineTo(x + left.x * SHIP_RADIUS, y + left.y * SHIP_RADIUS);
    ctx.lineTo(x + right.x * SHIP_RADIUS, y + right.y * SHIP_RADIUS);
    ctx.closePath();
    ctx.stroke();

    if (ship.thrusting) {
      const tail = fromAngle(a + Math.PI);
      const fl = fromAngle(a + (Math.PI * 5) / 6);
      const fr = fromAngle(a - (Math.PI * 5) / 6);
      ctx.beginPath();
      ctx.moveTo(x + fl.x * SHIP_RADIUS * 0.7, y + fl.y * SHIP_RADIUS * 0.7);
      ctx.lineTo(x + tail.x * SHIP_RADIUS * 1.6, y + tail.y * SHIP_RADIUS * 1.6);
      ctx.lineTo(x + fr.x * SHIP_RADIUS * 0.7, y + fr.y * SHIP_RADIUS * 0.7);
      ctx.stroke();
    }
  }

  private drawAsteroidWrapped(a: Asteroid, w: number, h: number): void {
    this.wrapped(a.pos.x, a.pos.y, a.radius, w, h, (ox, oy) =>
      this.drawAsteroid(a, ox, oy),
    );
  }

  private drawAsteroid(a: Asteroid, x: number, y: number): void {
    const { ctx } = this;
    const n = a.shape.length;
    ctx.beginPath();
    for (let i = 0; i < n; i++) {
      const ang = a.angle + (i / n) * Math.PI * 2;
      const r = a.radius * (a.shape[i] as number);
      const px = x + Math.cos(ang) * r;
      const py = y + Math.sin(ang) * r;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.closePath();
    ctx.stroke();
  }

  private drawDotWrapped(x: number, y: number, r: number, w: number, h: number): void {
    this.wrapped(x, y, r, w, h, (ox, oy) => {
      this.ctx.beginPath();
      this.ctx.arc(ox, oy, r, 0, Math.PI * 2);
      this.ctx.fill();
    });
  }

  private drawHud(state: GameState): void {
    const { ctx } = this;
    ctx.fillStyle = "#fff";
    ctx.font = "20px monospace";
    ctx.textAlign = "left";
    ctx.fillText(`SCORE ${state.score}`, 16, 28);
    ctx.fillText(`WAVE ${state.wave}`, 16, 52);

    ctx.textAlign = "right";
    ctx.fillText(`LIVES ${Math.max(0, state.lives)}`, state.width - 16, 28);
    // Little ship icons for remaining lives.
    for (let i = 0; i < Math.max(0, state.lives); i++) {
      this.drawShip(
        { angle: -Math.PI / 2 } as Ship,
        state.width - 24 - i * 22,
        62,
      );
    }
    ctx.textAlign = "left";
  }

  private drawCenterText(title: string, subtitle: string, w: number, h: number): void {
    const { ctx } = this;
    ctx.fillStyle = "#fff";
    ctx.textAlign = "center";
    ctx.font = "56px monospace";
    ctx.fillText(title, w / 2, h / 2 - 10);
    ctx.font = "22px monospace";
    ctx.fillText(subtitle, w / 2, h / 2 + 34);
    ctx.textAlign = "left";
  }
}
