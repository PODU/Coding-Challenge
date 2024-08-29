/**
 * One colour per segment, with no two neighbours (including last and first)
 * the same. Hues are spaced evenly so every colour is distinct; lightness
 * alternates so neighbours also differ in brightness.
 */
export function assignColors(n: number): string[] {
  if (n <= 0) return [];
  if (n === 1) return [hsl(210, 70, 55)];

  const out: string[] = [];
  for (let i = 0; i < n; i++) {
    const hue = Math.round((i * 360) / n) % 360;
    // With odd n the last and first share a lightness, but their hues differ.
    const light = i % 2 === 0 ? 58 : 48;
    out.push(hsl(hue, 68, light));
  }
  return out;
}

function hsl(h: number, s: number, l: number): string {
  return `hsl(${h}, ${s}%, ${l}%)`;
}
