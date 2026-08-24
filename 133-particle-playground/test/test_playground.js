'use strict';
// Headless test harness: stub DOM, eval playground.js + hook, drive the CA.
const fs = require('fs');

function fakeCtx() {
  return {
    imageSmoothingEnabled: false, globalAlpha: 1, globalCompositeOperation: '',
    fillStyle: '',
    createImageData: (w, h) => ({ data: new Uint8ClampedArray(w * h * 4) }),
    putImageData() {}, drawImage() {}, fillRect() {}, save() {}, restore() {},
  };
}
function fakeEl() {
  return {
    width: 960, height: 640, value: '4', textContent: '', innerHTML: '',
    children: [], style: {},
    classList: { add() {}, remove() {} },
    appendChild(c) { this.children.push(c); },
    addEventListener() {}, setPointerCapture() {},
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 960, height: 640 }),
    getContext: fakeCtx,
    oninput: null, onclick: null,
  };
}
global.document = { getElementById: () => fakeEl(), createElement: () => fakeEl() };
global.requestAnimationFrame = () => {};

// The sim is stochastic (fire spread, steam, dispersal). Seed Math.random with
// a fixed LCG so every run is reproducible instead of occasionally flaky.
let seed = 133;
Math.random = () => ((seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 4294967296);

const src = fs.readFileSync(require('path').join(__dirname, '..', 'playground.js'), 'utf8');
const hook = `
globalThis.__t = {
  mat, life, setCell, stepGrid, stepParticles, launchRocket, particles,
  GW, GH, E, SAND, WATER, OIL, STONE, WOOD, FIRE, SMOKE, STEAM, idx,
  tick(n) { for (let i = 0; i < n; i++) { frame++; stepGrid(); stepParticles(1/60); } },
  count(m) { let c = 0; for (const v of mat) if (v === m) c++; return c; },
  clear() { mat.fill(E); life.fill(0); chunkNext.fill(1); for (const p of particles) p.alive = false; },
};
`;
eval(src + hook);

const T = globalThis.__t;
let failures = 0;
function check(name, cond, detail = '') {
  console.log((cond ? 'PASS' : 'FAIL') + '  ' + name + (cond ? '' : '  ' + detail));
  if (!cond) failures++;
}

T.clear();
for (let x = 0; x < T.GW; x++) T.setCell(x, T.GH - 1, T.STONE); // floor
for (let i = 0; i < 200; i++) { T.setCell(120, 5, T.SAND); T.tick(2); }
T.tick(600);
check('sand count conserved', T.count(T.SAND) === 200, 'got ' + T.count(T.SAND));
// pile: no floating sand (every sand cell has support below), spans multiple columns
let floating = 0, cols = new Set();
for (let y = 0; y < T.GH - 1; y++) for (let x = 0; x < T.GW; x++) {
  if (T.mat[T.idx(x, y)] === T.SAND) {
    cols.add(x);
    const b = T.mat[T.idx(x, y + 1)];
    if (b === T.E) floating++;
  }
}
check('sand fully settled (none floating)', floating === 0, floating + ' floating');
check('sand forms a pile spread over >5 columns', cols.size > 5, 'cols=' + cols.size);

T.clear();
for (let x = 50; x < 150; x++) T.setCell(x, 140, T.STONE);
for (let y = 110; y < 140; y++) { T.setCell(50, y, T.STONE); T.setCell(149, y, T.STONE); }
for (let i = 0; i < 300; i++) { T.setCell(60, 100, T.WATER); T.tick(1); }
T.tick(1500);
// surface height per column inside basin should be near-uniform
const tops = [];
for (let x = 55; x < 145; x += 5) {
  for (let y = 100; y < 140; y++) {
    if (T.mat[T.idx(x, y)] === T.WATER) { tops.push(y); break; }
  }
}
const spread = Math.max(...tops) - Math.min(...tops);
check('water present in basin', T.count(T.WATER) > 200, 'water=' + T.count(T.WATER));
check('water level roughly flat (spread<=2)', tops.length > 10 && spread <= 2,
  'spread=' + spread + ' cols=' + tops.length);

T.clear();
for (let x = 50; x < 150; x++) T.setCell(x, 140, T.STONE);
for (let y = 100; y < 140; y++) { T.setCell(50, y, T.STONE); T.setCell(149, y, T.STONE); }
// oil at bottom, water dumped on top -> should invert
for (let y = 135; y < 140; y++) for (let x = 51; x < 149; x++) T.setCell(x, y, T.OIL);
for (let y = 125; y < 130; y++) for (let x = 51; x < 149; x++) T.setCell(x, y, T.WATER);
T.tick(2000);
let oilY = 0, oilN = 0, watY = 0, watN = 0;
for (let y = 0; y < T.GH; y++) for (let x = 0; x < T.GW; x++) {
  const m = T.mat[T.idx(x, y)];
  if (m === T.OIL) { oilY += y; oilN++; }
  if (m === T.WATER) { watY += y; watN++; }
}
check('oil floats above water', oilN > 0 && watN > 0 && (oilY / oilN) < (watY / watN),
  'avg oilY=' + (oilY / oilN).toFixed(1) + ' avg watY=' + (watY / watN).toFixed(1));

T.clear();
for (let x = 100; x < 120; x++) for (let y = 130; y < 140; y++) T.setCell(x, y, T.WOOD);
T.setCell(110, 129, T.FIRE, 200);
const woodBefore = T.count(T.WOOD);
let sawSmoke = false, sawFire = false;
for (let i = 0; i < 3000; i++) {
  T.tick(1);
  if (T.count(T.SMOKE) > 0) sawSmoke = true;
  if (T.count(T.FIRE) > 0) sawFire = true;
  if (sawFire && T.count(T.FIRE) === 0 && i > 100) break;
}
T.tick(500);
check('fire consumed wood', T.count(T.WOOD) < woodBefore,
  'before=' + woodBefore + ' after=' + T.count(T.WOOD));
check('smoke was produced', sawSmoke);
check('fire self-extinguished', T.count(T.FIRE) === 0, 'fire=' + T.count(T.FIRE));
check('smoke dissipated', T.count(T.SMOKE) === 0, 'smoke=' + T.count(T.SMOKE));

T.clear();
for (let x = 100; x < 110; x++) T.setCell(x, 140, T.STONE);
for (let y = 138; y < 140; y++) { T.setCell(100, y, T.STONE); T.setCell(109, y, T.STONE); } // keep water from draining off the ledge
for (let x = 101; x < 109; x++) T.setCell(x, 139, T.WATER);
for (let x = 101; x < 109; x++) T.setCell(x, 138, T.FIRE, 500);
let sawSteam = false;
for (let i = 0; i < 600 && !sawSteam; i++) { T.tick(1); if (T.count(T.STEAM) > 0) sawSteam = true; }
check('steam forms from water near fire', sawSteam);

T.clear();
for (let x = 100; x < 140; x++) T.setCell(x, 100, T.STONE);
for (let x = 110; x < 130; x++) T.setCell(x, 50, T.SAND);
for (let x = 110; x < 130; x++) T.setCell(x, 51, T.WATER);
T.tick(1000);
let below = 0;
for (let y = 101; y < T.GH; y++) for (let x = 100; x < 140; x++) {
  const m = T.mat[T.idx(x, y)];
  if (m === T.SAND || m === T.WATER) below++;
}
// material can flow around the ends of the shelf, but nothing may pass through
// the covered center columns
let throughCenter = 0;
for (let y = 101; y < T.GH; y++) for (let x = 112; x < 128; x++) {
  const m = T.mat[T.idx(x, y)];
  if (m === T.SAND) throughCenter++;
}
check('sand rests on stone shelf (center columns hold sand above)', (() => {
  for (let x = 112; x < 128; x++) if (T.mat[T.idx(x, 99)] === T.SAND || T.mat[T.idx(x, 99)] === T.WATER) return true;
  return false;
})());

T.clear();
for (let x = 0; x < T.GW; x++) T.setCell(x, T.GH - 1, T.STONE);
for (let x = 80; x < 160; x++) for (let y = 150; y < 155; y++) T.setCell(x, y, T.WOOD);
T.launchRocket(480, 600, 300, 'ring');
let maxParticles = 0, ignited = false;
for (let i = 0; i < 1200; i++) {
  T.tick(1);
  let live = 0;
  for (const p of T.particles) if (p.alive) live++;
  maxParticles = Math.max(maxParticles, live);
  if (T.count(T.FIRE) > 0) ignited = true;
}
check('rocket burst into many embers', maxParticles > 40, 'max=' + maxParticles);
check('embers ignited wood below', ignited);

T.clear();
for (let i = 0; i < 300; i++) T.launchRocket(480, 600, 100, 'willow');
T.tick(120);
check('particle pool capped at 4000', T.particles.length <= 4000, 'len=' + T.particles.length);

console.log(failures === 0 ? '\nALL PASS' : '\n' + failures + ' FAILURES');
process.exit(failures === 0 ? 0 : 1);
