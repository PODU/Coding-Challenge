/**
 * Block helpers with no I/O: type defaults, cleaning up marks, and building
 * the tree.
 */
import type {
  Block,
  BlockNode,
  BlockProps,
  BlockType,
  Mark,
} from '../../../shared/types.js';

/** Soft cap on nesting depth to bound recursive render. */
export const MAX_DEPTH = 50;

/** Hard cap on a single block's text length. */
export const MAX_TEXT_LENGTH = 100_000;

export function defaultPropsForType(type: BlockType): BlockProps {
  if (type === 'to_do') return { checked: false };
  return {};
}

/** `divider` carries no text. */
export function textForType(type: BlockType, text: string): string {
  return type === 'divider' ? '' : text;
}

/**
 * Clamp marks to [0, textLength], drop empty/invalid ranges, and merge
 * overlapping or touching ranges that share the same styling.
 */
export function normalizeMarks(marks: Mark[], textLength: number): Mark[] {
  const cleaned: Mark[] = [];
  for (const m of marks) {
    const start = Math.max(0, Math.min(m.start, textLength));
    const end = Math.max(0, Math.min(m.end, textLength));
    if (end <= start) continue;
    const norm: Mark = { start, end };
    if (m.bold) norm.bold = true;
    if (m.italic) norm.italic = true;
    if (!norm.bold && !norm.italic) continue;
    cleaned.push(norm);
  }

  cleaned.sort((a, b) => a.start - b.start || a.end - b.end);

  const merged: Mark[] = [];
  for (const m of cleaned) {
    const last = merged[merged.length - 1];
    if (
      last &&
      !!last.bold === !!m.bold &&
      !!last.italic === !!m.italic &&
      m.start <= last.end
    ) {
      last.end = Math.max(last.end, m.end);
    } else {
      merged.push({ ...m });
    }
  }
  return merged;
}

/**
 * Assemble flat blocks of one page into an ordered nested tree. Children are
 * linked via `parentId`; siblings sorted by `position`. Any block whose
 * `parentId` does not resolve is treated as a top-level node.
 */
export function buildTree(blocks: Block[]): BlockNode[] {
  const byId = new Map<string, BlockNode>();
  for (const b of blocks) {
    byId.set(b.id, { ...b, children: [] });
  }
  const roots: BlockNode[] = [];
  for (const node of byId.values()) {
    const parent = node.parentId ? byId.get(node.parentId) : undefined;
    if (parent) parent.children.push(node);
    else roots.push(node);
  }
  const sortRec = (nodes: BlockNode[]): void => {
    nodes.sort((a, b) => a.position - b.position);
    for (const n of nodes) sortRec(n.children);
  };
  sortRec(roots);
  return roots;
}

/** Depth-first list of a block id and all of its descendants (subtree). */
export function collectSubtreeIds(blocks: Block[], rootId: string): string[] {
  const childrenByParent = new Map<string, Block[]>();
  for (const b of blocks) {
    if (b.parentId) {
      const arr = childrenByParent.get(b.parentId) ?? [];
      arr.push(b);
      childrenByParent.set(b.parentId, arr);
    }
  }
  const ids: string[] = [];
  const visit = (id: string): void => {
    ids.push(id);
    for (const child of childrenByParent.get(id) ?? []) visit(child.id);
  };
  visit(rootId);
  return ids;
}

/** Depth of a block within the page tree (root blocks are depth 0). */
export function depthOf(blocks: Block[], blockId: string | null): number {
  const byId = new Map(blocks.map((b) => [b.id, b]));
  let depth = 0;
  let cur = blockId ? byId.get(blockId) : undefined;
  while (cur && cur.parentId) {
    depth += 1;
    cur = byId.get(cur.parentId);
    if (depth > MAX_DEPTH * 2) break; // guard against corrupt cycles
  }
  return depth;
}

/**
 * True if `candidateParentId` is `blockId` itself or one of its descendants —
 * i.e. moving `blockId` under it would create a cycle.
 */
export function wouldCreateCycle(
  blocks: Block[],
  blockId: string,
  candidateParentId: string | null,
): boolean {
  if (candidateParentId === null) return false;
  if (candidateParentId === blockId) return true;
  const byId = new Map(blocks.map((b) => [b.id, b]));
  let cur = byId.get(candidateParentId);
  let guard = 0;
  while (cur) {
    if (cur.id === blockId) return true;
    if (!cur.parentId) break;
    cur = byId.get(cur.parentId);
    if (++guard > MAX_DEPTH * 2) break;
  }
  return false;
}
