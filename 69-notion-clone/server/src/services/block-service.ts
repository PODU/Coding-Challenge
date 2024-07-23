import type { Block, BlockNode, BlockType } from '../../../shared/types.js';
import { AppError } from '../errors.js';
import type { Store } from '../store.js';
import { newId } from '../util/crypto.js';
import {
  MAX_DEPTH,
  MAX_TEXT_LENGTH,
  buildTree,
  collectSubtreeIds,
  defaultPropsForType,
  depthOf,
  normalizeMarks,
  textForType,
  wouldCreateCycle,
} from '../domain/blocks.js';
import { positionAfter, positionAtEnd } from '../domain/ordering.js';
import type {
  CreateBlockInput,
  MoveBlockInput,
  UpdateBlockInput,
} from '../domain/schemas.js';

export class BlockService {
  constructor(private readonly store: Store) {}

  /** Page owned by `userId`, else 404 (existence not leaked). */
  private ownedPage(userId: string, pageId: string) {
    const page = this.store.getPage(pageId);
    if (!page || page.userId !== userId) throw AppError.notFound('Page not found');
    return page;
  }

  /** Block whose page is owned by `userId`, else 404. */
  private ownedBlock(userId: string, blockId: string): Block {
    const block = this.store.getBlock(blockId);
    if (!block) throw AppError.notFound('Block not found');
    const page = this.store.getPage(block.pageId);
    if (!page || page.userId !== userId) throw AppError.notFound('Block not found');
    return block;
  }

  private touchPage(pageId: string): void {
    const page = this.store.getPage(pageId);
    if (page) {
      page.updatedAt = new Date().toISOString();
      this.store.updatePage(page);
    }
  }

  private siblings(pageId: string, parentId: string | null, excludeId?: string): Block[] {
    return this.store
      .listBlocksByPage(pageId)
      .filter((b) => b.parentId === parentId && b.id !== excludeId);
  }

  listTree(userId: string, pageId: string): BlockNode[] {
    this.ownedPage(userId, pageId);
    return buildTree(this.store.listBlocksByPage(pageId));
  }

  create(userId: string, pageId: string, input: CreateBlockInput): Block {
    this.ownedPage(userId, pageId);
    const parentId = input.parentId ?? null;

    // The parent must exist, be on the same page, and not be nested too deep.
    if (parentId !== null) {
      const parent = this.store.getBlock(parentId);
      if (!parent || parent.pageId !== pageId) {
        throw AppError.validation('parentId must reference a block on this page');
      }
      if (depthOf(this.store.listBlocksByPage(pageId), parentId) + 1 > MAX_DEPTH) {
        throw AppError.validation('Maximum nesting depth exceeded');
      }
    }

    return this.store.transaction(() => {
      const siblings = this.siblings(pageId, parentId);
      let position: number;
      if (input.afterBlockId) {
        const after = siblings.find((s) => s.id === input.afterBlockId);
        if (!after) {
          throw AppError.validation('afterBlockId must be a sibling in this group');
        }
        position = positionAfter(siblings, input.afterBlockId, (r) =>
          this.persistRenormalized(r),
        );
      } else {
        position = positionAtEnd(siblings);
      }

      const now = new Date().toISOString();
      const text = textForType(input.type, input.text ?? '');
      const block: Block = {
        id: newId(),
        pageId,
        parentId,
        type: input.type,
        text,
        marks: normalizeMarks(input.marks ?? [], text.length),
        props: { ...defaultPropsForType(input.type), ...(input.props ?? {}) },
        position,
        createdAt: now,
        updatedAt: now,
      };
      this.store.insertBlock(block);
      this.touchPage(pageId);
      return block;
    });
  }

  update(userId: string, blockId: string, input: UpdateBlockInput): Block {
    const block = this.ownedBlock(userId, blockId);
    if (input.text !== undefined) {
      if (input.text.length > MAX_TEXT_LENGTH) {
        throw AppError.validation('Block text too long');
      }
      block.text = textForType(block.type, input.text);
    }
    if (input.marks !== undefined) {
      block.marks = normalizeMarks(input.marks, block.text.length);
    } else if (input.text !== undefined) {
      // The text changed, so the marks may now run past its end.
      block.marks = normalizeMarks(block.marks, block.text.length);
    }
    if (input.props !== undefined) {
      block.props = { ...block.props, ...input.props };
    }
    block.updatedAt = new Date().toISOString();
    this.store.updateBlock(block);
    this.touchPage(block.pageId);
    return block;
  }

  changeType(userId: string, blockId: string, type: BlockType): Block {
    const block = this.ownedBlock(userId, blockId);
    block.type = type;
    block.text = textForType(type, block.text);
    block.marks = normalizeMarks(block.marks, block.text.length);
    block.props = defaultPropsForType(type); // reset props to type defaults
    block.updatedAt = new Date().toISOString();
    this.store.updateBlock(block);
    this.touchPage(block.pageId);
    return block;
  }

  move(userId: string, blockId: string, input: MoveBlockInput): Block {
    const block = this.ownedBlock(userId, blockId);
    const pageId = block.pageId;
    const newParentId =
      input.parentId === undefined ? block.parentId : input.parentId;

    if (newParentId !== null) {
      const parent = this.store.getBlock(newParentId);
      if (!parent || parent.pageId !== pageId) {
        throw AppError.validation('parentId must reference a block on this page');
      }
      const all = this.store.listBlocksByPage(pageId);
      if (wouldCreateCycle(all, blockId, newParentId)) {
        throw AppError.conflict('Cannot move a block under itself or its descendant');
      }
      if (depthOf(all, newParentId) + 1 > MAX_DEPTH) {
        throw AppError.validation('Maximum nesting depth exceeded');
      }
    }
    if (input.afterBlockId && input.afterBlockId === blockId) {
      throw AppError.validation('afterBlockId cannot be the block itself');
    }

    return this.store.transaction(() => {
      const siblings = this.siblings(pageId, newParentId, blockId);
      if (input.afterBlockId) {
        const after = siblings.find((s) => s.id === input.afterBlockId);
        if (!after) {
          throw AppError.validation('afterBlockId must be a sibling in the target group');
        }
      }
      const position = positionAfter(
        siblings,
        input.afterBlockId ?? null,
        (r) => this.persistRenormalized(r),
      );
      const fresh = this.store.getBlock(blockId)!;
      fresh.parentId = newParentId;
      fresh.position = position;
      fresh.updatedAt = new Date().toISOString();
      this.store.updateBlock(fresh);
      this.touchPage(pageId);
      return fresh;
    });
  }

  delete(userId: string, blockId: string): void {
    const block = this.ownedBlock(userId, blockId);
    this.store.transaction(() => {
      const ids = collectSubtreeIds(
        this.store.listBlocksByPage(block.pageId),
        blockId,
      );
      for (const id of ids) this.store.deleteBlock(id);
      this.touchPage(block.pageId);
    });
  }

  /**
   * Apply a list of structural ops atomically and return
   * the resulting block tree. Either all ops apply or none do.
   */
  batch(
    userId: string,
    pageId: string,
    ops: Array<Record<string, unknown>>,
  ): BlockNode[] {
    this.ownedPage(userId, pageId);
    return this.store.transaction(() => {
      for (const op of ops) {
        const kind = op['op'];
        const id = typeof op['id'] === 'string' ? (op['id'] as string) : '';
        switch (kind) {
          case 'create':
            this.create(userId, pageId, op as unknown as CreateBlockInput);
            break;
          case 'update':
            this.update(userId, id, op as unknown as UpdateBlockInput);
            break;
          case 'changeType':
            this.changeType(userId, id, op['type'] as BlockType);
            break;
          case 'move':
            this.move(userId, id, op as unknown as MoveBlockInput);
            break;
          case 'delete':
            this.delete(userId, id);
            break;
          default:
            throw AppError.validation(`Unknown batch op: ${String(kind)}`);
        }
      }
      return buildTree(this.store.listBlocksByPage(pageId));
    });
  }

  private persistRenormalized(renumbered: Block[]): void {
    for (const b of renumbered) this.store.updateBlock(b);
  }
}
