import {
  parseNames,
  shuffle,
  sortNames,
  removeAt,
  buildSegments,
  planSpin,
  resolveWinner,
  mulberry32,
  randomSeed,
  encodeWheel,
  decodeWheel,
  type Segment,
} from "./core/index.js";
import { WheelRenderer } from "./render/WheelRenderer.js";
import { SpinAnimator } from "./render/SpinAnimator.js";

const STORAGE_KEY = "wheel-of-names:v1";

interface AdvancedEntry {
  color?: string;
  weight?: number;
}

const $ = <T extends HTMLElement>(id: string): T => {
  const el = document.getElementById(id);
  if (!el) throw new Error(`missing element #${id}`);
  return el as T;
};

const namesEl = $<HTMLTextAreaElement>("names");
const shuffleBtn = $<HTMLButtonElement>("shuffle");
const sortBtn = $<HTMLButtonElement>("sort");
const spinBtn = $<HTMLButtonElement>("spin");
const removeWinnerEl = $<HTMLInputElement>("remove-winner");
const countEl = $<HTMLParagraphElement>("count");
const winnerEl = $<HTMLDivElement>("winner");
const advToggle = $<HTMLButtonElement>("advanced-toggle");
const advPanel = $<HTMLDivElement>("advanced");
const advRows = $<HTMLDivElement>("advanced-rows");
const canvas = $<HTMLCanvasElement>("wheel");

const renderer = new WheelRenderer(canvas);
const animator = new SpinAnimator();

let wheelAngle = 0;
let segments: Segment[] = [];
let advanced = false;
/** Colour and weight overrides, indexed like the current names. */
let advEntries: AdvancedEntry[] = [];

function sizeCanvas(): void {
  const dpr = window.devicePixelRatio || 1;
  const cssW = canvas.clientWidth || 560;
  canvas.width = Math.round(cssW * dpr);
  canvas.height = Math.round(cssW * dpr);
  rebuildAndDraw();
}

function currentNames(): string[] {
  return parseNames(namesEl.value);
}

function rebuildAndDraw(): void {
  const names = currentNames();
  if (advanced) {
    const colors = names.map((_, i) => advEntries[i]?.color ?? "");
    const weights = names.map((_, i) => advEntries[i]?.weight ?? 1);
    const allColored = colors.every((c) => c !== "");
    segments = buildSegments(names, {
      weights,
      ...(allColored ? { colors } : {}),
    });
  } else {
    segments = buildSegments(names);
  }
  countEl.textContent = names.length === 0 ? "No names yet." : `${names.length} name(s).`;
  spinBtn.disabled = names.length === 0 || animator.isRunning;
  renderer.draw(segments, wheelAngle);
}

function setNames(names: string[]): void {
  namesEl.value = names.join("\n");
  persist();
  if (advanced) syncAdvancedRows();
  rebuildAndDraw();
}

function syncAdvancedRows(): void {
  const names = currentNames();
  advEntries = names.map((_, i) => advEntries[i] ?? {});
  advRows.innerHTML = "";
  names.forEach((name, i) => {
    const row = document.createElement("div");
    row.className = "adv-row";

    const label = document.createElement("input");
    label.type = "text";
    label.value = name;
    label.readOnly = true;

    const color = document.createElement("input");
    color.type = "color";
    color.value = advEntries[i].color ?? hslToHex(`hsl(${Math.round((i * 360) / Math.max(1, names.length))}, 68%, 53%)`);
    color.addEventListener("input", () => {
      advEntries[i] = { ...advEntries[i], color: color.value };
      rebuildAndDraw();
    });
    // Store the default so buildSegments sees a colour for every row.
    advEntries[i] = { ...advEntries[i], color: advEntries[i].color ?? color.value };

    const weight = document.createElement("input");
    weight.type = "number";
    weight.min = "0.1";
    weight.step = "0.1";
    weight.value = String(advEntries[i].weight ?? 1);
    weight.addEventListener("input", () => {
      const w = parseFloat(weight.value);
      advEntries[i] = { ...advEntries[i], weight: Number.isFinite(w) && w > 0 ? w : 1 };
      rebuildAndDraw();
    });

    row.append(label, color, weight);
    advRows.appendChild(row);
  });
}

function spin(): void {
  if (segments.length === 0 || animator.isRunning) return;
  winnerEl.classList.remove("show");
  winnerEl.textContent = "";

  const rng = mulberry32(randomSeed());
  const plan = planSpin(segments, rng, wheelAngle);

  spinBtn.disabled = true;
  shuffleBtn.disabled = sortBtn.disabled = true;

  animator.spin(
    plan,
    wheelAngle,
    (angle) => {
      wheelAngle = angle;
      renderer.draw(segments, wheelAngle);
    },
    () => {
      // Read the winner off where the wheel actually stopped.
      const idx = resolveWinner(segments, wheelAngle);
      announceWinner(idx);
      shuffleBtn.disabled = sortBtn.disabled = false;
      spinBtn.disabled = segments.length === 0;

      if (removeWinnerEl.checked) {
        const names = removeAt(currentNames(), idx);
        if (advanced) advEntries = removeAt(advEntries, idx);
        setNames(names);
      }
    },
  );
}

function announceWinner(index: number): void {
  const seg = segments[index];
  if (!seg) return;
  winnerEl.textContent = `🎉 ${seg.label}`;
  winnerEl.classList.add("show");
}

function persist(): void {
  try {
    const names = currentNames();
    const state = advanced
      ? {
          names,
          weights: names.map((_, i) => advEntries[i]?.weight ?? 1),
          colors: names.map((_, i) => advEntries[i]?.color ?? ""),
        }
      : { names };
    localStorage.setItem(STORAGE_KEY, encodeWheel(state));
  } catch {
    /* storage unavailable; not worth failing over */
  }
}

function restore(): void {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return;
    const state = decodeWheel(raw);
    namesEl.value = state.names.join("\n");
    if (state.weights || state.colors) {
      advEntries = state.names.map((_, i) => ({
        weight: state.weights?.[i],
        color: state.colors?.[i] && state.colors[i] !== "" ? state.colors[i] : undefined,
      }));
    }
  } catch {
    /* ignore malformed storage */
  }
}

shuffleBtn.addEventListener("click", () => {
  setNames(shuffle(currentNames(), mulberry32(randomSeed())));
});
sortBtn.addEventListener("click", () => setNames(sortNames(currentNames())));
spinBtn.addEventListener("click", spin);
namesEl.addEventListener("input", () => {
  persist();
  if (advanced) syncAdvancedRows();
  rebuildAndDraw();
});
advToggle.addEventListener("click", () => {
  advanced = !advanced;
  advPanel.classList.toggle("hidden", !advanced);
  advToggle.textContent = advanced ? "Advanced ✓" : "Advanced…";
  if (advanced) syncAdvancedRows();
  rebuildAndDraw();
});

window.addEventListener("resize", sizeCanvas);

restore();
sizeCanvas();

function hslToHex(hsl: string): string {
  const m = /hsl\(\s*([\d.]+)\s*,\s*([\d.]+)%\s*,\s*([\d.]+)%/.exec(hsl);
  if (!m) return "#888888";
  const h = parseFloat(m[1]) / 360;
  const s = parseFloat(m[2]) / 100;
  const l = parseFloat(m[3]) / 100;
  const f = (n: number): number => {
    const k = (n + h * 12) % 12;
    const a = s * Math.min(l, 1 - l);
    return l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
  };
  const to = (x: number): string =>
    Math.round(x * 255)
      .toString(16)
      .padStart(2, "0");
  return `#${to(f(0))}${to(f(8))}${to(f(4))}`;
}
