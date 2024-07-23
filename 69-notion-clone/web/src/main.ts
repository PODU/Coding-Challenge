import type {
  Block,
  BlockNode,
  BlockType,
  Page,
  PageDetail,
} from '../../shared/types.js';
import { Api, ApiError } from './api.js';
import { h, clear, focusEnd, caretAtStart } from './dom.js';
import { marksToHtml, htmlToMarks } from './richtext.js';

const TYPE_LABELS: Record<BlockType, string> = {
  paragraph: 'Text',
  heading_1: 'Heading 1',
  heading_2: 'Heading 2',
  heading_3: 'Heading 3',
  bulleted_list_item: 'Bulleted list',
  numbered_list_item: 'Numbered list',
  to_do: 'To-do',
  quote: 'Quote',
  divider: 'Divider',
};
const LIST_TYPES: BlockType[] = ['bulleted_list_item', 'numbered_list_item', 'to_do'];

class App {
  private api = new Api();
  private root: HTMLElement;
  private pages: Page[] = [];
  private detail: PageDetail | null = null;
  private currentPageId: string | null = null;
  private saveTimers = new Map<string, number>();

  constructor(root: HTMLElement) {
    this.root = root;
  }

  async start(): Promise<void> {
    if (this.api.isAuthed) {
      try {
        await this.api.me();
        await this.showApp();
        return;
      } catch {
        this.api.setToken(null);
      }
    }
    this.showAuth();
  }

  private showAuth(): void {
    clear(this.root);
    let mode: 'login' | 'register' = 'login';
    const email = h('input', { type: 'email', placeholder: 'you@example.com', class: 'field' });
    const password = h('input', { type: 'password', placeholder: 'Password (min 6)', class: 'field' });
    const error = h('div', { class: 'auth-error' });
    const submit = h('button', { class: 'btn primary' }, ['Sign in']);
    const toggle = h('a', { class: 'auth-toggle', href: '#' }, ['Need an account? Register']);

    const setMode = (m: 'login' | 'register'): void => {
      mode = m;
      submit.textContent = m === 'login' ? 'Sign in' : 'Create account';
      toggle.textContent = m === 'login' ? 'Need an account? Register' : 'Have an account? Sign in';
    };
    toggle.addEventListener('click', (e) => {
      e.preventDefault();
      setMode(mode === 'login' ? 'register' : 'login');
    });

    const doAuth = async (): Promise<void> => {
      error.textContent = '';
      try {
        const res =
          mode === 'login'
            ? await this.api.login(email.value, password.value)
            : await this.api.register(email.value, password.value);
        this.api.setToken(res.token);
        await this.showApp();
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : 'Something went wrong';
      }
    };
    submit.addEventListener('click', () => void doAuth());
    password.addEventListener('keydown', (e) => {
      if ((e as KeyboardEvent).key === 'Enter') void doAuth();
    });

    this.root.append(
      h('div', { class: 'auth-wrap' }, [
        h('div', { class: 'auth-card' }, [
          h('h1', { class: 'logo' }, ['📝 Notion Clone']),
          email,
          password,
          submit,
          error,
          toggle,
        ]),
      ]),
    );
  }

  private async showApp(): Promise<void> {
    clear(this.root);
    const sidebar = h('aside', { class: 'sidebar', id: 'sidebar' });
    const main = h('main', { class: 'editor', id: 'editor' });
    this.root.append(h('div', { class: 'layout' }, [sidebar, main]));
    await this.refreshPages();
    if (this.pages.length === 0) {
      await this.createPage(null);
    } else {
      await this.openPage(this.currentPageId ?? this.pages[0].id);
    }
  }

  private async refreshPages(): Promise<void> {
    this.pages = await this.api.listPages();
    this.renderSidebar();
  }

  private renderSidebar(): void {
    const sidebar = document.getElementById('sidebar');
    if (!sidebar) return;
    clear(sidebar);
    sidebar.append(
      h('div', { class: 'sidebar-head' }, [
        h('span', { class: 'workspace' }, ['Workspace']),
        h('button', { class: 'btn small', onClick: () => void this.createPage(null) }, ['+ New']),
      ]),
    );
    const byParent = new Map<string | null, Page[]>();
    for (const p of this.pages) {
      const arr = byParent.get(p.parentPageId) ?? [];
      arr.push(p);
      byParent.set(p.parentPageId, arr);
    }
    for (const list of byParent.values()) list.sort((a, b) => a.position - b.position);

    const renderLevel = (parentId: string | null, depth: number): HTMLElement[] =>
      (byParent.get(parentId) ?? []).flatMap((page) => {
        const row = h('div', {
          class: `page-row${page.id === this.currentPageId ? ' active' : ''}`,
          style: `padding-left:${8 + depth * 14}px`,
        }, [
          h('span', { class: 'page-title', onClick: () => void this.openPage(page.id) }, [page.title]),
          h('span', { class: 'page-actions' }, [
            h('button', { class: 'icon-btn', title: 'Add subpage', onClick: () => void this.createPage(page.id) }, ['+']),
            h('button', { class: 'icon-btn', title: 'Duplicate', onClick: () => void this.duplicatePage(page.id) }, ['⧉']),
            h('button', { class: 'icon-btn', title: 'Delete', onClick: () => void this.deletePage(page.id) }, ['🗑']),
          ]),
        ]);
        return [row, ...renderLevel(page.id, depth + 1)];
      });

    sidebar.append(h('div', { class: 'page-list' }, renderLevel(null, 0)));
  }

  private async createPage(parentPageId: string | null): Promise<void> {
    const page = await this.api.createPage({ parentPageId });
    await this.refreshPages();
    await this.openPage(page.id);
  }
  private async duplicatePage(id: string): Promise<void> {
    const page = await this.api.duplicatePage(id);
    await this.refreshPages();
    await this.openPage(page.id);
  }
  private async deletePage(id: string): Promise<void> {
    if (!confirm('Delete this page and all sub-pages?')) return;
    await this.api.deletePage(id);
    if (this.currentPageId === id) this.currentPageId = null;
    await this.refreshPages();
    if (this.pages.length === 0) await this.createPage(null);
    else await this.openPage(this.currentPageId ?? this.pages[0].id);
  }

  private async openPage(id: string): Promise<void> {
    this.currentPageId = id;
    this.detail = await this.api.getPage(id);
    this.renderSidebar();
    this.renderEditor();
  }

  private renderEditor(focusBlockId?: string): void {
    const editor = document.getElementById('editor');
    if (!editor || !this.detail) return;
    clear(editor);

    const title = h('div', {
      class: 'page-title-input',
      contentEditable: 'true',
      spellcheck: false,
    });
    title.textContent = this.detail.page.title;
    title.addEventListener('blur', () => void this.saveTitle(title.textContent ?? ''));
    title.addEventListener('keydown', (e) => {
      if ((e as KeyboardEvent).key === 'Enter') {
        e.preventDefault();
        const first = editor.querySelector<HTMLElement>('.block-content');
        if (first) focusEnd(first);
      }
    });

    const blocks = h('div', { class: 'blocks' });
    for (const node of this.detail.blocks) blocks.append(this.renderBlock(node, 0));

    editor.append(h('div', { class: 'page' }, [title, blocks]));
    if (focusBlockId) {
      const el = editor.querySelector<HTMLElement>(`[data-block="${focusBlockId}"] .block-content`);
      if (el) focusEnd(el);
    }
  }

  private numberFor(node: BlockNode): number {
    const siblings = this.siblingsOf(node.parentId);
    let n = 0;
    for (const s of siblings) {
      if (s.type === 'numbered_list_item') n++;
      if (s.id === node.id) break;
    }
    return n;
  }

  private renderBlock(node: BlockNode, depth: number): HTMLElement {
    const wrap = h('div', { class: 'block', dataset: { block: node.id } });
    const row = h('div', { class: `block-row type-${node.type}` });

    if (node.type === 'divider') {
      row.append(h('hr', { class: 'divider' }));
    } else {
      // marker
      if (node.type === 'bulleted_list_item') row.append(h('span', { class: 'marker' }, ['•']));
      else if (node.type === 'numbered_list_item') row.append(h('span', { class: 'marker' }, [`${this.numberFor(node)}.`]));
      else if (node.type === 'to_do') {
        const cb = h('input', { type: 'checkbox', class: 'todo-check' }) as HTMLInputElement;
        cb.checked = !!node.props.checked;
        cb.addEventListener('change', () => void this.api.updateBlock(node.id, { props: { checked: cb.checked } }).then(() => {
          content.classList.toggle('done', cb.checked);
        }));
        row.append(cb);
      }

      const content = h('div', {
        class: `block-content${node.type === 'to_do' && node.props.checked ? ' done' : ''}`,
        contentEditable: 'true',
        spellcheck: false,
        dataset: { placeholder: TYPE_LABELS[node.type] },
      });
      content.innerHTML = marksToHtml(node.text, node.marks);
      content.addEventListener('input', () => this.onInput(node, content));
      content.addEventListener('keydown', (e) => void this.onKeydown(e as KeyboardEvent, node, content));
      row.append(content);
    }

    wrap.append(row);
    if (node.children.length > 0) {
      const childWrap = h('div', { class: 'children' });
      for (const child of node.children) childWrap.append(this.renderBlock(child, depth + 1));
      wrap.append(childWrap);
    }
    return wrap;
  }

  private onInput(node: Block, el: HTMLElement): void {
    // slash menu trigger
    if (el.textContent === '/') {
      this.openSlashMenu(node, el);
      return;
    }
    const { text, marks } = htmlToMarks(el);
    const prev = this.saveTimers.get(node.id);
    if (prev) window.clearTimeout(prev);
    const timer = window.setTimeout(() => {
      void this.api.updateBlock(node.id, { text, marks }).catch(() => {});
      this.saveTimers.delete(node.id);
    }, 400);
    this.saveTimers.set(node.id, timer);
  }

  private async flushSave(node: Block, el: HTMLElement): Promise<void> {
    const prev = this.saveTimers.get(node.id);
    if (prev) window.clearTimeout(prev);
    this.saveTimers.delete(node.id);
    const { text, marks } = htmlToMarks(el);
    await this.api.updateBlock(node.id, { text, marks }).catch(() => {});
  }

  private async onKeydown(e: KeyboardEvent, node: BlockNode, el: HTMLElement): Promise<void> {
    const meta = e.ctrlKey || e.metaKey;
    if (meta && (e.key === 'b' || e.key === 'i')) {
      e.preventDefault();
      document.execCommand(e.key === 'b' ? 'bold' : 'italic');
      this.onInput(node, el);
      return;
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      await this.handleEnter(node, el);
    } else if (e.key === 'Tab') {
      e.preventDefault();
      await this.flushSave(node, el);
      if (e.shiftKey) await this.outdent(node);
      else await this.indent(node);
    } else if (e.key === 'Backspace') {
      if ((el.textContent ?? '').length === 0 && caretAtStart(el) && node.children.length === 0) {
        e.preventDefault();
        await this.deleteBlock(node);
      }
    }
  }

  private async handleEnter(node: BlockNode, el: HTMLElement): Promise<void> {
    await this.flushSave(node, el);
    const isList = LIST_TYPES.includes(node.type);
    const empty = (el.textContent ?? '').length === 0;
    if (isList && empty) {
      // exit the list: convert to paragraph
      await this.api.changeBlockType(node.id, 'paragraph');
      await this.reloadFocus(node.id);
      return;
    }
    const newType: BlockType = isList ? node.type : 'paragraph';
    const created = await this.api.createBlock(this.currentPageId!, {
      type: newType,
      parentId: node.parentId,
      afterBlockId: node.id,
    });
    await this.reloadFocus(created.id);
  }

  private async indent(node: BlockNode): Promise<void> {
    const siblings = this.siblingsOf(node.parentId);
    const idx = siblings.findIndex((s) => s.id === node.id);
    if (idx <= 0) return; // nothing to nest under
    const prev = siblings[idx - 1];
    const lastChild = prev.children[prev.children.length - 1];
    await this.api.moveBlock(node.id, {
      parentId: prev.id,
      afterBlockId: lastChild ? lastChild.id : null,
    });
    await this.reloadFocus(node.id);
  }

  private async outdent(node: BlockNode): Promise<void> {
    if (node.parentId === null) return;
    const parent = this.findBlock(node.parentId);
    if (!parent) return;
    await this.api.moveBlock(node.id, {
      parentId: parent.parentId,
      afterBlockId: parent.id,
    });
    await this.reloadFocus(node.id);
  }

  private async deleteBlock(node: BlockNode): Promise<void> {
    const order = this.flatten();
    const idx = order.findIndex((b) => b.id === node.id);
    const prevId = idx > 0 ? order[idx - 1].id : undefined;
    await this.api.deleteBlock(node.id);
    await this.reloadFocus(prevId);
  }

  private async changeType(node: Block, type: BlockType): Promise<void> {
    await this.api.changeBlockType(node.id, type);
    await this.reloadFocus(type === 'divider' ? undefined : node.id);
  }

  private async saveTitle(text: string): Promise<void> {
    if (!this.currentPageId) return;
    await this.api.updatePage(this.currentPageId, { title: text });
    await this.refreshPages();
  }

  private async reloadFocus(focusId?: string): Promise<void> {
    if (!this.currentPageId) return;
    this.detail = await this.api.getPage(this.currentPageId);
    this.renderEditor(focusId);
  }

  private openSlashMenu(node: Block, el: HTMLElement): void {
    document.querySelector('.slash-menu')?.remove();
    const menu = h('div', { class: 'slash-menu' });
    (Object.keys(TYPE_LABELS) as BlockType[]).forEach((type) => {
      menu.append(
        h('div', { class: 'slash-item', onClick: async () => {
          menu.remove();
          el.textContent = '';
          await this.changeType(node, type);
        } }, [TYPE_LABELS[type]]),
      );
    });
    const rect = el.getBoundingClientRect();
    menu.style.position = 'absolute';
    menu.style.left = `${rect.left}px`;
    menu.style.top = `${rect.bottom + window.scrollY}px`;
    document.body.append(menu);
    const close = (ev: MouseEvent): void => {
      if (!menu.contains(ev.target as Node)) {
        menu.remove();
        document.removeEventListener('mousedown', close);
      }
    };
    setTimeout(() => document.addEventListener('mousedown', close), 0);
  }

  private siblingsOf(parentId: string | null): BlockNode[] {
    if (!this.detail) return [];
    if (parentId === null) return this.detail.blocks;
    const parent = this.findBlock(parentId);
    return parent ? parent.children : [];
  }
  private findBlock(id: string): BlockNode | null {
    const visit = (nodes: BlockNode[]): BlockNode | null => {
      for (const n of nodes) {
        if (n.id === id) return n;
        const found = visit(n.children);
        if (found) return found;
      }
      return null;
    };
    return this.detail ? visit(this.detail.blocks) : null;
  }
  private flatten(): BlockNode[] {
    const out: BlockNode[] = [];
    const visit = (nodes: BlockNode[]): void => {
      for (const n of nodes) {
        out.push(n);
        visit(n.children);
      }
    };
    if (this.detail) visit(this.detail.blocks);
    return out;
  }
}

const root = document.getElementById('app');
if (root) void new App(root).start();
