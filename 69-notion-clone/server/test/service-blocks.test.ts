import { describe, it, expect, beforeEach } from 'vitest';
import { makeCore } from './helpers.js';
import type { BlockService } from '../src/services/block-service.js';

describe('BlockService (core tree operations)', () => {
  let blocks: BlockService;
  let userId: string;
  let pageId: string;

  beforeEach(() => {
    const core = makeCore();
    blocks = core.bundle.services.blocks;
    userId = core.userId;
    pageId = core.pageId;
  });

  const order = () =>
    blocks.listTree(userId, pageId).map((n) => n.text);

  it('creates blocks at end and preserves sibling order', () => {
    blocks.create(userId, pageId, { type: 'paragraph', text: 'A' });
    blocks.create(userId, pageId, { type: 'paragraph', text: 'B' });
    // page seeds one empty paragraph; so order is ['', 'A', 'B']
    expect(order()).toEqual(['', 'A', 'B']);
  });

  it('creates with afterBlockId in the middle', () => {
    const a = blocks.create(userId, pageId, { type: 'paragraph', text: 'A' });
    blocks.create(userId, pageId, { type: 'paragraph', text: 'C' });
    blocks.create(userId, pageId, {
      type: 'paragraph',
      text: 'B',
      afterBlockId: a.id,
    });
    expect(order()).toEqual(['', 'A', 'B', 'C']);
  });

  it('indent (move) reparents and keeps the subtree', () => {
    const a = blocks.create(userId, pageId, { type: 'bulleted_list_item', text: 'A' });
    const b = blocks.create(userId, pageId, { type: 'bulleted_list_item', text: 'B' });
    blocks.move(userId, b.id, { parentId: a.id });
    const tree = blocks.listTree(userId, pageId);
    const aNode = tree.find((n) => n.id === a.id)!;
    expect(aNode.children.map((c) => c.text)).toEqual(['B']);
  });

  it('rejects moving a block under its own descendant (cycle)', () => {
    const a = blocks.create(userId, pageId, { type: 'bulleted_list_item', text: 'A' });
    const b = blocks.create(userId, pageId, { type: 'bulleted_list_item', text: 'B' });
    blocks.move(userId, b.id, { parentId: a.id });
    expect(() => blocks.move(userId, a.id, { parentId: b.id })).toThrowError(
      /under itself or its descendant/,
    );
  });

  it('subtree delete removes descendants but not siblings', () => {
    const a = blocks.create(userId, pageId, { type: 'bulleted_list_item', text: 'A' });
    const child = blocks.create(userId, pageId, {
      type: 'bulleted_list_item',
      text: 'A.1',
      parentId: a.id,
    });
    const sibling = blocks.create(userId, pageId, {
      type: 'bulleted_list_item',
      text: 'B',
    });
    blocks.delete(userId, a.id);
    const tree = blocks.listTree(userId, pageId);
    const ids = tree.map((n) => n.id);
    expect(ids).toContain(sibling.id);
    expect(ids).not.toContain(a.id);
    expect(tree.flatMap((n) => n.children).map((c) => c.id)).not.toContain(child.id);
  });

  it('change type preserves text/marks, resets props, keeps children', () => {
    const a = blocks.create(userId, pageId, {
      type: 'paragraph',
      text: 'Hello',
      marks: [{ start: 0, end: 5, bold: true }],
    });
    blocks.create(userId, pageId, {
      type: 'bulleted_list_item',
      text: 'child',
      parentId: a.id,
    });
    const changed = blocks.changeType(userId, a.id, 'to_do');
    expect(changed.type).toBe('to_do');
    expect(changed.text).toBe('Hello');
    expect(changed.marks).toEqual([{ start: 0, end: 5, bold: true }]);
    expect(changed.props).toEqual({ checked: false });
    const tree = blocks.listTree(userId, pageId);
    const aNode = tree.find((n) => n.id === a.id)!;
    expect(aNode.children).toHaveLength(1);
  });

  it('update toggles a to_do checkbox and clamps marks on text change', () => {
    const t = blocks.create(userId, pageId, { type: 'to_do', text: 'task' });
    const checked = blocks.update(userId, t.id, { props: { checked: true } });
    expect(checked.props.checked).toBe(true);
    const shorter = blocks.update(userId, t.id, {
      text: 'ab',
      marks: [{ start: 0, end: 10, italic: true }],
    });
    expect(shorter.marks).toEqual([{ start: 0, end: 2, italic: true }]);
  });

  it('move to start (no afterBlockId) places before siblings', () => {
    const a = blocks.create(userId, pageId, { type: 'paragraph', text: 'A' });
    const b = blocks.create(userId, pageId, { type: 'paragraph', text: 'B' });
    blocks.move(userId, b.id, { parentId: null, afterBlockId: null });
    const roots = blocks.listTree(userId, pageId);
    expect(roots[0].id).toBe(b.id);
    expect(roots.map((n) => n.id)).toContain(a.id);
  });

  it('batch applies multiple ops atomically', () => {
    const a = blocks.create(userId, pageId, { type: 'paragraph', text: 'A' });
    const tree = blocks.batch(userId, pageId, [
      { op: 'update', id: a.id, text: 'A!' },
      { op: 'create', type: 'paragraph', text: 'Z' },
    ]);
    const texts = tree.map((n) => n.text);
    expect(texts).toContain('A!');
    expect(texts).toContain('Z');
  });

  it('rolls back a batch when one op fails', () => {
    const a = blocks.create(userId, pageId, { type: 'paragraph', text: 'A' });
    expect(() =>
      blocks.batch(userId, pageId, [
        { op: 'update', id: a.id, text: 'changed' },
        { op: 'delete', id: 'non-existent-id' },
      ]),
    ).toThrow();
    // first op must have been rolled back
    const aNode = blocks.listTree(userId, pageId).find((n) => n.id === a.id)!;
    expect(aNode.text).toBe('A');
  });
});
