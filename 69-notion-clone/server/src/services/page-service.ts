import type {
  Block,
  BlockNode,
  Page,
  PageDetail,
} from '../../../shared/types.js';
import { AppError } from '../errors.js';
import type { Store } from '../store.js';
import { newId } from '../util/crypto.js';
import { buildTree } from '../domain/blocks.js';
import { positionAfter, positionAtEnd } from '../domain/ordering.js';
import type {
  CreatePageInput,
  ReorderPageInput,
  UpdatePageInput,
} from '../domain/schemas.js';

const DEFAULT_TITLE = 'Untitled';

function cleanTitle(title: string | undefined): string {
  const t = (title ?? '').trim();
  return t.length === 0 ? DEFAULT_TITLE : t;
}

export class PageService {
  constructor(private readonly store: Store) {}

  private owned(userId: string, pageId: string): Page {
    const page = this.store.getPage(pageId);
    if (!page || page.userId !== userId) throw AppError.notFound('Page not found');
    return page;
  }

  private siblings(userId: string, parentPageId: string | null, excludeId?: string): Page[] {
    return this.store
      .listPagesByUser(userId)
      .filter((p) => p.parentPageId === parentPageId && p.id !== excludeId);
  }

  /** Reject making `pageId` a descendant of itself (page-tree cycle). */
  private assertNoCycle(
    userId: string,
    pageId: string,
    candidateParentId: string | null,
  ): void {
    if (candidateParentId === null) return;
    if (candidateParentId === pageId) {
      throw AppError.conflict('A page cannot be nested under itself');
    }
    const byId = new Map(this.store.listPagesByUser(userId).map((p) => [p.id, p]));
    let cur = byId.get(candidateParentId);
    if (!cur) throw AppError.validation('parentPageId not found');
    let guard = 0;
    while (cur) {
      if (cur.id === pageId) {
        throw AppError.conflict('A page cannot be nested under its own descendant');
      }
      if (!cur.parentPageId) break;
      cur = byId.get(cur.parentPageId);
      if (++guard > 10_000) break;
    }
  }

  list(userId: string): Page[] {
    return this.store
      .listPagesByUser(userId)
      .sort((a, b) => a.position - b.position);
  }

  create(userId: string, input: CreatePageInput): Page {
    const parentPageId = input.parentPageId ?? null;
    if (parentPageId !== null) this.owned(userId, parentPageId);

    return this.store.transaction(() => {
      const now = new Date().toISOString();
      const page: Page = {
        id: newId(),
        userId,
        parentPageId,
        title: cleanTitle(input.title),
        position: positionAtEnd(this.siblings(userId, parentPageId)),
        createdAt: now,
        updatedAt: now,
      };
      this.store.insertPage(page);
      // Empty pages start with one paragraph block so the editor has a focus target.
      this.store.insertBlock({
        id: newId(),
        pageId: page.id,
        parentId: null,
        type: 'paragraph',
        text: '',
        marks: [],
        props: {},
        position: 1,
        createdAt: now,
        updatedAt: now,
      });
      return page;
    });
  }

  get(userId: string, pageId: string): PageDetail {
    const page = this.owned(userId, pageId);
    const blocks: BlockNode[] = buildTree(this.store.listBlocksByPage(pageId));
    return { page, blocks };
  }

  update(userId: string, pageId: string, input: UpdatePageInput): Page {
    const page = this.owned(userId, pageId);
    return this.store.transaction(() => {
      if (input.title !== undefined) page.title = cleanTitle(input.title);
      if (input.parentPageId !== undefined) {
        if (input.parentPageId !== null) this.owned(userId, input.parentPageId);
        this.assertNoCycle(userId, pageId, input.parentPageId);
        if (input.parentPageId !== page.parentPageId) {
          page.parentPageId = input.parentPageId;
          page.position = positionAtEnd(
            this.siblings(userId, input.parentPageId, pageId),
          );
        }
      }
      if (input.position !== undefined) page.position = input.position;
      page.updatedAt = new Date().toISOString();
      this.store.updatePage(page);
      return page;
    });
  }

  reorder(userId: string, pageId: string, input: ReorderPageInput): Page {
    const page = this.owned(userId, pageId);
    const newParentId =
      input.parentPageId === undefined ? page.parentPageId : input.parentPageId;
    if (newParentId !== null) {
      this.owned(userId, newParentId);
      this.assertNoCycle(userId, pageId, newParentId);
    }
    if (input.afterPageId && input.afterPageId === pageId) {
      throw AppError.validation('afterPageId cannot be the page itself');
    }
    return this.store.transaction(() => {
      const siblings = this.siblings(userId, newParentId, pageId);
      if (input.afterPageId) {
        const after = siblings.find((s) => s.id === input.afterPageId);
        if (!after) {
          throw AppError.validation('afterPageId must be a sibling in the target group');
        }
      }
      const position = positionAfter(
        siblings,
        input.afterPageId ?? null,
        (r) => {
          for (const p of r) this.store.updatePage(p);
        },
      );
      page.parentPageId = newParentId;
      page.position = position;
      page.updatedAt = new Date().toISOString();
      this.store.updatePage(page);
      return page;
    });
  }

  duplicate(userId: string, pageId: string): Page {
    const source = this.owned(userId, pageId);
    return this.store.transaction(() => {
      const now = new Date().toISOString();
      // Include the source so the copy lands immediately after it in sidebar order.
      const siblings = this.siblings(userId, source.parentPageId);
      const position = positionAfter(siblings, source.id, (r) => {
        for (const p of r) this.store.updatePage(p);
      });
      const copy: Page = {
        id: newId(),
        userId,
        parentPageId: source.parentPageId,
        title: `Copy of ${source.title}`,
        position,
        createdAt: now,
        updatedAt: now,
      };
      this.store.insertPage(copy);
      this.copyBlocks(source.id, copy.id);
      return copy;
    });
  }

  /** Deep-copy a page's whole block tree with fresh ids, preserving structure. */
  private copyBlocks(srcPageId: string, dstPageId: string): void {
    const src = this.store.listBlocksByPage(srcPageId);
    const idMap = new Map<string, string>();
    for (const b of src) idMap.set(b.id, newId());
    for (const b of src) {
      const copy: Block = {
        ...b,
        id: idMap.get(b.id)!,
        pageId: dstPageId,
        parentId: b.parentId ? idMap.get(b.parentId) ?? null : null,
        marks: structuredClone(b.marks),
        props: structuredClone(b.props),
      };
      this.store.insertBlock(copy);
    }
  }

  delete(userId: string, pageId: string): void {
    this.owned(userId, pageId);
    this.store.transaction(() => {
      const all = this.store.listPagesByUser(userId);
      const childrenByParent = new Map<string, Page[]>();
      for (const p of all) {
        if (p.parentPageId) {
          const arr = childrenByParent.get(p.parentPageId) ?? [];
          arr.push(p);
          childrenByParent.set(p.parentPageId, arr);
        }
      }
      const toDelete: string[] = [];
      const visit = (id: string): void => {
        toDelete.push(id);
        for (const child of childrenByParent.get(id) ?? []) visit(child.id);
      };
      visit(pageId);
      for (const id of toDelete) {
        this.store.deleteBlocksByPage(id);
        this.store.deletePage(id);
      }
    });
  }
}
