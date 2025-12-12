/**
 * The script text, plus the content height in CSS px that the renderer
 * measures after layout.
 */

export class ScriptModel {
  private _text = "";
  private _contentHeight = 0;

  constructor(text = "") {
    this._text = normalizeNewlines(text);
  }

  get text(): string {
    return this._text;
  }

  get contentHeight(): number {
    return this._contentHeight;
  }

  setText(text: string): void {
    this._text = normalizeNewlines(text);
  }

  setContentHeight(px: number): void {
    this._contentHeight = Number.isFinite(px) && px > 0 ? px : 0;
  }

  get isEmpty(): boolean {
    return this._text.trim().length === 0;
  }
}

/** Convert CRLF and lone CR line endings to LF. */
export function normalizeNewlines(s: string): string {
  return s.replace(/\r\n?/g, "\n");
}
