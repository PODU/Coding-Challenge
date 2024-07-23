import type { Mark } from '../../shared/types.js';

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

/** Render (text, marks) to safe HTML with <strong>/<em> spans. */
export function marksToHtml(text: string, marks: Mark[]): string {
  if (text.length === 0) return '';
  const bold = new Array<boolean>(text.length).fill(false);
  const italic = new Array<boolean>(text.length).fill(false);
  for (const m of marks) {
    for (let i = Math.max(0, m.start); i < Math.min(text.length, m.end); i++) {
      if (m.bold) bold[i] = true;
      if (m.italic) italic[i] = true;
    }
  }
  let html = '';
  let i = 0;
  while (i < text.length) {
    const b = bold[i];
    const it = italic[i];
    let j = i;
    while (j < text.length && bold[j] === b && italic[j] === it) j++;
    let seg = escapeHtml(text.slice(i, j));
    if (b) seg = `<strong>${seg}</strong>`;
    if (it) seg = `<em>${seg}</em>`;
    html += seg;
    i = j;
  }
  return html;
}

/** Read a contentEditable element back into (text, marks). */
export function htmlToMarks(el: HTMLElement): { text: string; marks: Mark[] } {
  let text = '';
  const marks: Mark[] = [];
  const walk = (node: Node, bold: boolean, italic: boolean): void => {
    node.childNodes.forEach((child) => {
      if (child.nodeType === Node.TEXT_NODE) {
        const t = child.textContent ?? '';
        if (t.length > 0) {
          if (bold || italic) {
            const mark: Mark = { start: text.length, end: text.length + t.length };
            if (bold) mark.bold = true;
            if (italic) mark.italic = true;
            marks.push(mark);
          }
          text += t;
        }
      } else if (child.nodeType === Node.ELEMENT_NODE) {
        const tag = (child as HTMLElement).tagName;
        if (tag === 'BR') {
          text += '\n';
          return;
        }
        const b = bold || tag === 'STRONG' || tag === 'B';
        const it = italic || tag === 'EM' || tag === 'I';
        walk(child, b, it);
      }
    });
  };
  walk(el, false, false);
  return { text, marks };
}
