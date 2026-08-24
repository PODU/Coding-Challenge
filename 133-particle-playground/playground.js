'use strict';
/*
 * Particle Playground — Coding Challenge #133
 * https://codingchallenges.substack.com/p/coding-challenge-133-particle-playground
 *
 * Two coupled systems:
 *  - a cellular-automaton grid (sand / water / oil / stone / wood / fire / smoke / steam)
 *  - a particle engine (sparks, firework rockets and embers) with gravity, wind,
 *    drag and grid collision. Embers ignite flammable grid cells.
 *
 * Simulation runs on a fixed 60 Hz timestep decoupled from the render loop.
 * Grid updates only touch active 16x16 chunks; settled regions sleep.
 */

// Grid constants
const GW = 240, GH = 160, CELL = 4;          // 240x160 cells, 4 px each
const STEP = 1 / 60;                          // fixed sim timestep (s)
const MAX_PARTICLES = 4000;

const E = 0, SAND = 1, WATER = 2, OIL = 3, STONE = 4,
      WOOD = 5, FIRE = 6, SMOKE = 7, STEAM = 8;

const BASE_COLOR = [
  [0, 0, 0],        // empty
  [214, 186, 118],  // sand
  [52, 120, 210],   // water
  [92, 74, 48],     // oil
  [128, 130, 134],  // stone
  [140, 90, 43],    // wood
  [255, 120, 20],   // fire (recoloured by life)
  [90, 90, 95],     // smoke
  [188, 205, 212],  // steam
];

const isGas    = m => m === SMOKE || m === STEAM;
const isLiquid = m => m === WATER || m === OIL;
const isSolidForParticle = m => m === SAND || m === STONE || m === WOOD;

// Grid state
const mat   = new Uint8Array(GW * GH);   // material per cell
const life  = new Uint16Array(GW * GH);  // countdown for fire/smoke/steam
const shade = new Uint8Array(GW * GH);   // per-cell tint for texture
const stamp = new Uint32Array(GW * GH);  // frame a cell was last updated

for (let i = 0; i < shade.length; i++) shade[i] = Math.random() * 255;

// Chunked activity tracking: only cells inside awake chunks are simulated.
const CS = 16, CW = Math.ceil(GW / CS), CH = Math.ceil(GH / CS);
let chunkCur  = new Uint8Array(CW * CH);
let chunkNext = new Uint8Array(CW * CH);
chunkCur.fill(1); // everything awake on first frame

function wakeChunk(cx, cy) {
  if (cx >= 0 && cx < CW && cy >= 0 && cy < CH) chunkNext[cy * CW + cx] = 1;
}
function wake(x, y) {
  const cx = (x / CS) | 0, cy = (y / CS) | 0;
  wakeChunk(cx, cy);
  const lx = x % CS, ly = y % CS;
  if (lx === 0)      wakeChunk(cx - 1, cy);
  if (lx === CS - 1) wakeChunk(cx + 1, cy);
  if (ly === 0)      wakeChunk(cx, cy - 1);
  if (ly === CS - 1) wakeChunk(cx, cy + 1);
}

const idx = (x, y) => y * GW + x;
const inBounds = (x, y) => x >= 0 && x < GW && y >= 0 && y < GH;
const cellAt = (x, y) => inBounds(x, y) ? mat[idx(x, y)] : STONE; // walls

let frame = 1;

function setCell(x, y, m, l = 0) {
  if (!inBounds(x, y)) return;
  const i = idx(x, y);
  mat[i] = m;
  life[i] = l;
  stamp[i] = frame;
  wake(x, y);
}

// Move the contents of (x,y) to (nx,ny); whatever was there goes to (x,y).
function swapCells(x, y, nx, ny) {
  const a = idx(x, y), b = idx(nx, ny);
  const m = mat[a], l = life[a], s = shade[a];
  mat[a] = mat[b]; life[a] = life[b]; shade[a] = shade[b];
  mat[b] = m;      life[b] = l;      shade[b] = s;
  stamp[a] = stamp[b] = frame;
  wake(x, y); wake(nx, ny);
}

// Cellular automaton rules

// Sand sinks through gases and liquids, otherwise falls straight or diagonally.
function updateSand(x, y) {
  const below = cellAt(x, y + 1);
  if (below === E || isGas(below) || isLiquid(below)) { swapCells(x, y, x, y + 1); return; }
  const dir = Math.random() < 0.5 ? 1 : -1;
  for (const d of [dir, -dir]) {
    const m = cellAt(x + d, y + 1);
    if (m === E || isGas(m) || isLiquid(m)) { swapCells(x, y, x + d, y + 1); return; }
  }
}

// Liquids fall, sink through lighter liquids, then disperse sideways.
// Water is denser than oil, which is how oil ends up floating.
function updateLiquid(x, y, me) {
  const denserThan = m => me === WATER && m === OIL;
  const passable   = m => m === E || isGas(m);

  const below = cellAt(x, y + 1);
  if (passable(below) || denserThan(below)) { swapCells(x, y, x, y + 1); return; }

  const dir = Math.random() < 0.5 ? 1 : -1;
  for (const d of [dir, -dir]) {
    const m = cellAt(x + d, y + 1);
    if (passable(m) || denserThan(m)) { swapCells(x, y, x + d, y + 1); return; }
  }

  // Horizontal dispersion: slide toward the farthest reachable empty cell.
  const disp = me === WATER ? 5 : 3;
  for (const d of [dir, -dir]) {
    let dest = 0;
    for (let i = 1; i <= disp; i++) {
      const m = cellAt(x + d * i, y);
      if (m === E) dest = i;
      else if (!isGas(m)) break;
    }
    if (dest > 0) { swapCells(x, y, x + d * dest, y); return; }
  }
}

const N8 = [[-1,-1],[0,-1],[1,-1],[-1,0],[1,0],[-1,1],[0,1],[1,1]];

function updateFire(x, y) {
  const i = idx(x, y);
  wake(x, y); // fire stays hot until it burns out
  shade[i] = Math.random() * 255; // flicker

  if (--life[i] === 0) {
    setCell(x, y, Math.random() < 0.5 ? SMOKE : E, 90 + Math.random() * 90);
    return;
  }

  for (const [dx, dy] of N8) {
    const nx = x + dx, ny = y + dy;
    const m = cellAt(nx, ny);
    if (m === WOOD && Math.random() < 0.04) setCell(nx, ny, FIRE, 120 + Math.random() * 120);
    else if (m === OIL && Math.random() < 0.25) setCell(nx, ny, FIRE, 60 + Math.random() * 60);
    else if (m === WATER) {
      if (Math.random() < 0.15) setCell(nx, ny, STEAM, 120 + Math.random() * 120);
      if (Math.random() < 0.2) { setCell(x, y, SMOKE, 60 + Math.random() * 60); return; }
    }
  }

  // Fire vents smoke into empty space above it.
  if (cellAt(x, y - 1) === E && Math.random() < 0.12)
    setCell(x, y - 1, SMOKE, 90 + Math.random() * 120);
}

// Smoke and steam drift upward and dissipate; steam may condense back to water.
function updateGas(x, y, me) {
  const i = idx(x, y);
  wake(x, y);

  if (--life[i] === 0) {
    setCell(x, y, me === STEAM && Math.random() < 0.35 ? WATER : E);
    return;
  }

  const dir = Math.random() < 0.5 ? 1 : -1;
  const tries = [[0, -1], [dir, -1], [-dir, -1], [dir, 0], [-dir, 0]];
  for (const [dx, dy] of tries) {
    if (cellAt(x + dx, y + dy) === E && Math.random() < 0.8) {
      swapCells(x, y, x + dx, y + dy);
      return;
    }
  }
}

let activeCellCount = 0;

function stepGrid() {
  // Swap chunk activity buffers; the new frame starts with everything asleep.
  const t = chunkCur; chunkCur = chunkNext; chunkNext = t;
  chunkNext.fill(0);
  activeCellCount = 0;

  const ltr = (frame & 1) === 0; // alternate scan direction to avoid bias

  // Bottom-up scan so falling material settles in a single pass.
  for (let y = GH - 1; y >= 0; y--) {
    const cy = (y / CS) | 0;
    for (let sx = 0; sx < GW; sx++) {
      const x = ltr ? sx : GW - 1 - sx;
      if (!chunkCur[cy * CW + ((x / CS) | 0)]) continue;
      const i = idx(x, y);
      const m = mat[i];
      if (m === E) continue;
      activeCellCount++;
      if (stamp[i] === frame) continue; // already moved this frame
      switch (m) {
        case SAND:  updateSand(x, y); break;
        case WATER:
        case OIL:   updateLiquid(x, y, m); break;
        case FIRE:  updateFire(x, y); break;
        case SMOKE:
        case STEAM: updateGas(x, y, m); break;
        // STONE and WOOD are static
      }
    }
  }
}

// Particle engine
// Types: 'spark' (plain physics demo), 'rocket', 'ember', 'trail'
const particles = [];
let liveParticles = 0;

const forces = { gravity: 320, wind: 0, drag: 0.35 };

function spawnParticle(p) {
  // Pooled: reuse a dead slot before growing the array.
  for (let i = 0; i < particles.length; i++) {
    if (!particles[i].alive) { Object.assign(particles[i], p, { alive: true }); return particles[i]; }
  }
  if (particles.length >= MAX_PARTICLES) return null;
  p.alive = true;
  particles.push(p);
  return p;
}

function makeParticle(type, x, y, vx, vy, lifeS, color, extra) {
  return spawnParticle(Object.assign({
    type, x, y, vx, vy,
    px: x, py: y,
    life: lifeS, maxLife: lifeS,
    color, size: 2,
    dragMul: 1, gravMul: 1,
    strobe: false, splitAt: 0, burstY: 0, pattern: '', hue: 0,
    clear: false, // rockets: true once the particle has reached open air
  }, extra || {}));
}

function fireworkColor() {
  const hue = (Math.random() * 360) | 0;
  return `hsl(${hue},100%,65%)`;
}

function launchRocket(x, y, burstHeight, pattern) {
  makeParticle('rocket', x, y, (Math.random() - 0.5) * 20, -300, 6, '#ffd9a0', {
    burstY: Math.max(12, y - burstHeight),
    pattern,
    gravMul: 0,   // rockets thrust against gravity; modelled as no net pull
    dragMul: 0,
    size: 2.5,
  });
}

function explode(x, y, pattern) {
  const color = fireworkColor();
  const spawnRing = (n, speed, lifeS, extra) => {
    for (let k = 0; k < n; k++) {
      const a = (k / n) * Math.PI * 2 + Math.random() * 0.1;
      const s = speed * (0.85 + Math.random() * 0.3);
      makeParticle('ember', x, y, Math.cos(a) * s, Math.sin(a) * s,
        lifeS * (0.8 + Math.random() * 0.4), color, extra);
    }
  };
  switch (pattern) {
    case 'ring':      spawnRing(64, 170, 1.6); break;
    case 'willow':    spawnRing(80, 150, 3.2, { dragMul: 3.2, gravMul: 1.4, color: '#ffd700' }); break;
    case 'crossette': spawnRing(12, 180, 2.2, { splitAt: 0.55 }); break;
    case 'strobe':    spawnRing(56, 140, 2.6, { strobe: true, color: '#ffffff' }); break;
  }
}

function crossetteSplit(p) {
  for (let k = 0; k < 6; k++) {
    const a = Math.random() * Math.PI * 2;
    makeParticle('ember', p.x, p.y, Math.cos(a) * 90, Math.sin(a) * 90,
      0.9 + Math.random() * 0.4, p.color);
  }
}

function stepParticles(dt) {
  liveParticles = 0;
  for (const p of particles) {
    if (!p.alive) continue;
    liveParticles++;

    p.life -= dt;
    if (p.life <= 0) { p.alive = false; continue; }

    if (p.type === 'rocket') {
      if (Math.random() < 0.7)
        makeParticle('trail', p.x, p.y, (Math.random() - 0.5) * 30, 40, 0.4, '#a88', { gravMul: 0, size: 1.5 });
      if (p.y <= p.burstY || p.life < 0.2) {
        p.alive = false;
        explode(p.x, p.y, p.pattern);
        continue;
      }
    }

    if (p.splitAt && p.maxLife - p.life >= p.splitAt) {
      p.alive = false;
      crossetteSplit(p);
      continue;
    }

    // Forces: gravity down, wind sideways, drag against velocity.
    p.vx += forces.wind * dt;
    p.vy += forces.gravity * p.gravMul * dt;
    const dragK = forces.drag * p.dragMul * dt;
    p.vx -= p.vx * dragK;
    p.vy -= p.vy * dragK;

    p.px = p.x; p.py = p.y;
    p.x += p.vx * dt;
    p.y += p.vy * dt;

    // Canvas edges: bounce with damping so particles settle instead of jittering.
    const W = GW * CELL, H = GH * CELL;
    if (p.x < 0)  { p.x = 0;  p.vx = -p.vx * 0.5; }
    if (p.x > W)  { p.x = W;  p.vx = -p.vx * 0.5; }
    if (p.y < 0)  { p.y = 0;  p.vy = -p.vy * 0.3; }
    if (p.y > H - 1) {
      p.y = H - 1;
      p.vy = -p.vy * 0.45;
      p.vx *= 0.8;
      if (Math.abs(p.vy) < 20) p.vy = 0; // settled on floor
    }

    // Grid collision.
    const gx = (p.x / CELL) | 0, gy = (p.y / CELL) | 0;
    const m = cellAt(gx, gy);
    if (p.type === 'ember') {
      if (m === WOOD || m === OIL) {           // selective ignition
        setCell(gx, gy, FIRE, 120 + Math.random() * 120);
        p.alive = false;
        continue;
      }
      if (m === WATER) { p.alive = false; continue; } // fizzle
    }
    if (isSolidForParticle(m)) {
      if (p.type === 'rocket') {
        // A rocket launched from inside terrain tunnels out; once it has
        // reached open air, hitting a solid detonates it.
        if (p.clear) { p.alive = false; explode(p.x, p.y, p.pattern); }
        continue;
      }
      // Step back and bounce off whichever axis we entered through.
      const pgx = (p.px / CELL) | 0, pgy = (p.py / CELL) | 0;
      p.x = p.px; p.y = p.py;
      if (pgx !== gx) p.vx = -p.vx * 0.45;
      if (pgy !== gy) { p.vy = -p.vy * 0.45; p.vx *= 0.8; }
      if (Math.hypot(p.vx, p.vy) < 15) p.alive = false;
    } else if (p.type === 'rocket') {
      p.clear = true;
    }
  }
}

// Rendering
const view = document.getElementById('view');
const ctx = view.getContext('2d');
ctx.imageSmoothingEnabled = false;

const gridCanvas = document.createElement('canvas');
gridCanvas.width = GW; gridCanvas.height = GH;
const gctx = gridCanvas.getContext('2d');
const img = gctx.createImageData(GW, GH);
const px = img.data;

function renderGrid() {
  for (let i = 0, o = 0; i < GW * GH; i++, o += 4) {
    const m = mat[i];
    if (m === E) { px[o] = 8; px[o + 1] = 9; px[o + 2] = 14; px[o + 3] = 255; continue; }
    let [r, g, b] = BASE_COLOR[m];
    const t = 0.82 + (shade[i] / 255) * 0.18;
    if (m === FIRE) {
      // hotter fire is yellower
      const heat = Math.min(1, life[i] / 150);
      r = 255; g = 90 + 140 * heat * t; b = 20 * t;
    } else if (m === SMOKE || m === STEAM) {
      const fade = Math.min(1, life[i] / 60);
      r *= t * fade; g *= t * fade; b *= t * fade;
      r += 8 * (1 - fade); g += 9 * (1 - fade); b += 14 * (1 - fade);
    } else {
      r *= t; g *= t; b *= t;
    }
    px[o] = r; px[o + 1] = g; px[o + 2] = b; px[o + 3] = 255;
  }
  gctx.putImageData(img, 0, 0);
  ctx.drawImage(gridCanvas, 0, 0, view.width, view.height);
}

function renderParticles() {
  ctx.save();
  ctx.globalCompositeOperation = 'lighter';
  for (const p of particles) {
    if (!p.alive) continue;
    if (p.strobe && ((p.life * 14) | 0) % 2 === 0) continue; // flicker off-phase
    ctx.globalAlpha = Math.max(0, Math.min(1, p.life / p.maxLife));
    ctx.fillStyle = p.color;
    ctx.fillRect(p.x - p.size / 2, p.y - p.size / 2, p.size, p.size);
  }
  ctx.restore();
  ctx.globalAlpha = 1;
}

// Input & UI
const TOOLS = [
  { key: 'sand',     label: 'Sand',    m: SAND,  color: '#d6ba76' },
  { key: 'water',    label: 'Water',   m: WATER, color: '#3478d2' },
  { key: 'oil',      label: 'Oil',     m: OIL,   color: '#5c4a30' },
  { key: 'stone',    label: 'Stone',   m: STONE, color: '#808286' },
  { key: 'wood',     label: 'Wood',    m: WOOD,  color: '#8c5a2b' },
  { key: 'fire',     label: 'Fire',    m: FIRE,  color: '#ff7814' },
  { key: 'erase',    label: 'Erase',   m: E,     color: '#222' },
  { key: 'spark',    label: 'Sparks',  m: -1,    color: '#ffe28a' },
  { key: 'firework', label: 'Firework', m: -2,   color: '#ff5ad0' },
];
let tool = TOOLS[0];

const paletteEl = document.getElementById('palette');
for (const t of TOOLS) {
  const b = document.createElement('button');
  b.innerHTML = `<span class="swatch" style="background:${t.color}"></span>${t.label}`;
  b.onclick = () => {
    tool = t;
    [...paletteEl.children].forEach(c => c.classList.remove('sel'));
    b.classList.add('sel');
  };
  paletteEl.appendChild(b);
  if (t === tool) b.classList.add('sel');
}

const brushSlider = document.getElementById('brush');
const windSlider  = document.getElementById('wind');
const burstSlider = document.getElementById('burst');
const patternSel  = document.getElementById('pattern');
const bind = (slider, outId, fn) => {
  const out = document.getElementById(outId);
  const apply = () => { out.textContent = slider.value; fn(+slider.value); };
  slider.oninput = apply;
  apply();
};
bind(brushSlider, 'brushOut', () => {});
bind(windSlider,  'windOut',  v => { forces.wind = v; });
bind(burstSlider, 'burstOut', () => {});

let paused = false;
document.getElementById('pauseBtn').onclick = function () {
  paused = !paused;
  this.textContent = paused ? 'Resume' : 'Pause';
};
document.getElementById('clearBtn').onclick = () => {
  mat.fill(E); life.fill(0);
  for (const p of particles) p.alive = false;
  chunkNext.fill(1);
};

function paint(cx, cy) {
  const r = +brushSlider.value;
  for (let dy = -r; dy <= r; dy++) {
    for (let dx = -r; dx <= r; dx++) {
      if (dx * dx + dy * dy > r * r) continue;
      const x = cx + dx, y = cy + dy;
      if (!inBounds(x, y)) continue;
      if (tool.m === E) { setCell(x, y, E); continue; }
      if (Math.random() < 0.85 || tool.m === STONE || tool.m === WOOD) {
        if (mat[idx(x, y)] === E || tool.m === STONE || tool.m === WOOD) {
          const l = tool.m === FIRE ? 120 + Math.random() * 120 : 0;
          setCell(x, y, tool.m, l);
        }
      }
    }
  }
}

const mouse = { down: false, x: 0, y: 0, startX: 0, startY: 0 };

function canvasPos(ev) {
  const r = view.getBoundingClientRect();
  return {
    x: (ev.clientX - r.left) * (view.width / r.width),
    y: (ev.clientY - r.top) * (view.height / r.height),
  };
}

view.addEventListener('pointerdown', ev => {
  view.setPointerCapture(ev.pointerId);
  const p = canvasPos(ev);
  mouse.down = true;
  mouse.x = p.x; mouse.y = p.y;
  mouse.startX = p.x; mouse.startY = p.y;
  applyInput(); // paint immediately — a fast click may end before the next sim step
});
view.addEventListener('pointermove', ev => {
  const p = canvasPos(ev);
  if (mouse.down && tool.key !== 'firework') {
    // Interpolate along the drag path so fast drags leave a continuous stroke.
    const dist = Math.hypot(p.x - mouse.x, p.y - mouse.y);
    const steps = Math.max(1, Math.ceil(dist / (CELL * 2)));
    const sx = mouse.x, sy = mouse.y;
    for (let i = 1; i <= steps; i++) {
      mouse.x = sx + (p.x - sx) * (i / steps);
      mouse.y = sy + (p.y - sy) * (i / steps);
      applyInput();
    }
  }
  mouse.x = p.x; mouse.y = p.y;
});
view.addEventListener('pointerup', () => {
  if (mouse.down && tool.key === 'firework') {
    const drag = mouse.startY - mouse.y; // upward drag distance
    const burst = drag > 10 ? drag : +burstSlider.value;
    launchRocket(mouse.startX, mouse.startY, burst, patternSel.value);
  }
  mouse.down = false;
});
view.addEventListener('wheel', ev => {
  ev.preventDefault();
  const v = Math.max(1, Math.min(20, +brushSlider.value - Math.sign(ev.deltaY)));
  brushSlider.value = v;
  brushSlider.oninput();
}, { passive: false });

function applyInput() {
  if (!mouse.down) return;
  if (tool.key === 'firework') return; // launches on release
  if (tool.key === 'spark') {
    for (let k = 0; k < 6; k++) {
      makeParticle('spark', mouse.x, mouse.y,
        (Math.random() - 0.5) * 160, -Math.random() * 120,
        1.5 + Math.random() * 1.5, `hsl(${40 + Math.random() * 20},100%,70%)`);
    }
    return;
  }
  paint((mouse.x / CELL) | 0, (mouse.y / CELL) | 0);
}

// Main loop: fixed-timestep simulation, per-rAF rendering
const hud = document.getElementById('hud');
let last = performance.now(), acc = 0;
let fps = 0, fpsFrames = 0, fpsTime = 0;

function loop(now) {
  requestAnimationFrame(loop);
  let dt = (now - last) / 1000;
  last = now;
  if (dt > 0.25) dt = 0.25; // tab was backgrounded; don't spiral

  fpsFrames++; fpsTime += dt;
  if (fpsTime >= 0.5) { fps = Math.round(fpsFrames / fpsTime); fpsFrames = 0; fpsTime = 0; }

  acc += dt;
  let steps = 0;
  while (acc >= STEP && steps < 4) {
    applyInput(); // painting works while paused too
    if (!paused) {
      frame++;
      stepGrid();
      stepParticles(STEP);
    }
    acc -= STEP;
    steps++;
  }
  if (steps === 4) acc = 0; // running slow; drop the backlog

  renderGrid();
  renderParticles();

  hud.textContent =
    `fps ${fps}\n` +
    `particles ${liveParticles}\n` +
    `active cells ${activeCellCount}` +
    (paused ? '\nPAUSED' : '');
}

// A stone floor basin so liquids have something to pool in on first load.
for (let x = 40; x < 200; x++) setCell(x, GH - 8, STONE);
for (let y = GH - 30; y < GH - 8; y++) { setCell(40, y, STONE); setCell(199, y, STONE); }

requestAnimationFrame(loop);
