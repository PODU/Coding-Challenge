import { describe, it, expect } from 'vitest';
import {
  buildTree,
  collectSubtreeIds,
  depthOf,
  normalizeMarks,
  wouldCreateCycle,
  defaultPropsForType,
  textForType,
} from '../src/domain/blocks.js';
import { positionAfter, midpoint } from '../src/domain/ordering.js';
import type { Block } from '../../shared/types.js';

function block(partial: Partial<Block> & { id: string }): Block {
  return {
    pageId: 'p',
    parentId: null,
    type: 'paragraph',
    text: '',
    marks: [],
    props: {},
    position: 1,
    createdAt: 't',
    updatedAt: 't',
    ...partial,
  };
}

describe('ordering: fractional positions', () => {
  it('midpoint inserts strictly between neighbours', () => {
    expect(midpoint(null, null)).toBe(1);
    expect(midpoint(1, null)).toBe(2);
    expect(midpoint(null, 2)).toBe(1);
    expect(midpoint(2, 4)).toBe(3);
  });

  it('positionAfter places at start / middle / end keeping order increasing', () => {
    const sibs = [
      block({ id: 'a', position: 1 }),
      block({ id: 'b', position: 2 }),
      block({ id: 'c', position: 3 }),
    ];
    const noop = (): void => {};
    const atStart = positionAfter(sibs, null, noop);
    const afterA = positionAfter(sibs, 'a', noop);
    const afterC = positionAfter(sibs, 'c', noop);
    expect(atStart).toBeLessThan(1);
    expect(afterA).toBeGreaterThan(1);
    expect(afterA).toBeLessThan(2);
    expect(afterC).toBeGreaterThan(3);
  });

  it('renormalises when the midpoint underflows float precision', () => {
    const next = 1 + Number.EPSILON; // adjacent float to 1
    const sibs = [
      block({ id: 'a', position: 1 }),
      block({ id: 'b', position: next }),
    ];
    let renormalized: Block[] = [];
    const pos = positionAfter(sibs, 'a', (r) => {
      renormalized = r;
    });
    expect(renormalized.length).toBe(2); // group was reassigned
    expect(renormalized[0].position).toBe(1);
    expect(renormalized[1].position).toBe(2);
    expect(pos).toBeGreaterThan(1);
    expect(pos).toBeLessThan(2);
  });

  it('throws when afterId is not a sibling', () => {
    expect(() => positionAfter([block({ id: 'a' })], 'nope', () => {})).toThrow();
  });
});

describe('tree assembly', () => {
  it('links parentId into an ordered nested tree', () => {
    const blocks = [
      block({ id: 'root2', position: 2 }),
      block({ id: 'root1', position: 1 }),
      block({ id: 'child2', parentId: 'root1', position: 2 }),
      block({ id: 'child1', parentId: 'root1', position: 1 }),
    ];
    const tree = buildTree(blocks);
    expect(tree.map((n) => n.id)).toEqual(['root1', 'root2']);
    expect(tree[0].children.map((n) => n.id)).toEqual(['child1', 'child2']);
  });

  it('treats unresolved parents as roots (orphan safety)', () => {
    const tree = buildTree([block({ id: 'x', parentId: 'ghost' })]);
    expect(tree.map((n) => n.id)).toEqual(['x']);
  });
});

describe('subtree + cycle helpers', () => {
  const blocks = [
    block({ id: 'a' }),
    block({ id: 'b', parentId: 'a' }),
    block({ id: 'c', parentId: 'b' }),
    block({ id: 'd' }),
  ];

  it('collectSubtreeIds returns the node and all descendants only', () => {
    expect(collectSubtreeIds(blocks, 'a').sort()).toEqual(['a', 'b', 'c']);
    expect(collectSubtreeIds(blocks, 'b').sort()).toEqual(['b', 'c']);
    expect(collectSubtreeIds(blocks, 'd')).toEqual(['d']);
  });

  it('depthOf counts ancestors', () => {
    expect(depthOf(blocks, 'a')).toBe(0);
    expect(depthOf(blocks, 'c')).toBe(2);
  });

  it('wouldCreateCycle detects self and descendant targets', () => {
    expect(wouldCreateCycle(blocks, 'a', 'a')).toBe(true);
    expect(wouldCreateCycle(blocks, 'a', 'c')).toBe(true); // c is under a
    expect(wouldCreateCycle(blocks, 'a', 'd')).toBe(false);
    expect(wouldCreateCycle(blocks, 'a', null)).toBe(false);
  });
});

describe('marks + type defaults', () => {
  it('clamps, drops empty/style-less, and merges overlapping marks', () => {
    const out = normalizeMarks(
      [
        { start: -5, end: 4, bold: true },
        { start: 3, end: 100, bold: true },
        { start: 0, end: 0, italic: true },
        { start: 1, end: 2 },
      ],
      6,
    );
    expect(out).toEqual([{ start: 0, end: 6, bold: true }]);
  });

  it('keeps differing styles separate', () => {
    const out = normalizeMarks(
      [
        { start: 0, end: 2, bold: true },
        { start: 2, end: 4, italic: true },
      ],
      4,
    );
    expect(out).toHaveLength(2);
  });

  it('type defaults: to_do gets checked=false, divider clears text', () => {
    expect(defaultPropsForType('to_do')).toEqual({ checked: false });
    expect(defaultPropsForType('paragraph')).toEqual({});
    expect(textForType('divider', 'hi')).toBe('');
    expect(textForType('paragraph', 'hi')).toBe('hi');
  });
});
