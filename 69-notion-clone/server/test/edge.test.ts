import { describe, it, expect } from 'vitest';
import { makeCore } from './helpers.js';
import { makeTestApp } from '../src/app.js';
import { MAX_DEPTH } from '../src/domain/blocks.js';

describe('Edge cases', () => {
  it('rejects a cross-page parent for a block', () => {
    const { bundle, userId, pageId } = makeCore();
    const otherPage = bundle.services.pages.create(userId, { title: 'Other' });
    const otherBlock = bundle.services.blocks.create(userId, otherPage.id, {
      type: 'paragraph',
      text: 'x',
    });
    expect(() =>
      bundle.services.blocks.create(userId, pageId, {
        type: 'paragraph',
        text: 'y',
        parentId: otherBlock.id,
      }),
    ).toThrowError(/reference a block on this page/);
  });

  it('rejects nesting a page under itself (409)', () => {
    const { bundle, userId, pageId } = makeCore();
    expect(() =>
      bundle.services.pages.update(userId, pageId, { parentPageId: pageId }),
    ).toThrowError(/nested under itself/);
  });

  it('rejects nesting a page under its own descendant (409)', () => {
    const { bundle, userId, pageId } = makeCore();
    const child = bundle.services.pages.create(userId, { parentPageId: pageId });
    expect(() =>
      bundle.services.pages.update(userId, pageId, { parentPageId: child.id }),
    ).toThrowError(/descendant/);
  });

  it('enforces the maximum nesting depth', () => {
    const { bundle, userId, pageId } = makeCore();
    let parentId: string | null = null;
    // Build a chain whose deepest node sits at MAX_DEPTH; one more indent is rejected.
    for (let i = 0; i <= MAX_DEPTH; i++) {
      const b = bundle.services.blocks.create(userId, pageId, {
        type: 'bulleted_list_item',
        text: `d${i}`,
        parentId,
      });
      parentId = b.id;
    }
    expect(() =>
      bundle.services.blocks.create(userId, pageId, {
        type: 'bulleted_list_item',
        text: 'too deep',
        parentId,
      }),
    ).toThrowError(/depth/);
  });

  it('places a duplicated page immediately after the source', () => {
    const { bundle, userId, pageId } = makeCore();
    const source = bundle.services.pages.get(userId, pageId).page;
    const after = bundle.services.pages.create(userId, { title: 'After' });
    const dup = bundle.services.pages.duplicate(userId, pageId);
    expect(dup.position).toBeGreaterThan(source.position);
    expect(dup.position).toBeLessThan(after.position);
  });

  it('starts a freshly created page with one empty paragraph block', () => {
    const { bundle, userId } = makeCore();
    const page = bundle.services.pages.create(userId, {});
    const detail = bundle.services.pages.get(userId, page.id);
    expect(detail.blocks).toHaveLength(1);
    expect(detail.blocks[0].type).toBe('paragraph');
    expect(detail.blocks[0].text).toBe('');
  });

  it('isolates two test apps (hermetic stores)', () => {
    const one = makeTestApp();
    const two = makeTestApp();
    one.services.auth.register('only@one.com', 'secret123');
    expect(two.store.getUserByEmail('only@one.com')).toBeUndefined();
  });
});
