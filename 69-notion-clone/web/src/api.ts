import type {
  AuthResponse,
  Block,
  BlockType,
  Mark,
  Page,
  PageDetail,
  User,
} from '../../shared/types.js';

const TOKEN_KEY = 'notion_token';

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

export class Api {
  private token: string | null = localStorage.getItem(TOKEN_KEY);

  get isAuthed(): boolean {
    return this.token !== null;
  }

  setToken(token: string | null): void {
    this.token = token;
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers['content-type'] = 'application/json';
    if (this.token) headers['authorization'] = `Bearer ${this.token}`;
    const res = await fetch(`/api${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    if (res.status === 204) return undefined as T;
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      const err = (data as { error?: { code: string; message: string } }).error;
      throw new ApiError(res.status, err?.code ?? 'INTERNAL', err?.message ?? 'Request failed');
    }
    return data as T;
  }

  register(email: string, password: string): Promise<AuthResponse> {
    return this.request('POST', '/auth/register', { email, password });
  }
  login(email: string, password: string): Promise<AuthResponse> {
    return this.request('POST', '/auth/login', { email, password });
  }
  me(): Promise<{ user: User }> {
    return this.request('GET', '/auth/me');
  }

  listPages(): Promise<Page[]> {
    return this.request('GET', '/pages');
  }
  createPage(input: { title?: string; parentPageId?: string | null }): Promise<Page> {
    return this.request('POST', '/pages', input);
  }
  getPage(id: string): Promise<PageDetail> {
    return this.request('GET', `/pages/${id}`);
  }
  updatePage(id: string, input: { title?: string; parentPageId?: string | null }): Promise<Page> {
    return this.request('PATCH', `/pages/${id}`, input);
  }
  duplicatePage(id: string): Promise<Page> {
    return this.request('POST', `/pages/${id}/duplicate`);
  }
  reorderPage(id: string, input: { afterPageId?: string | null; parentPageId?: string | null }): Promise<Page> {
    return this.request('POST', `/pages/${id}/reorder`, input);
  }
  deletePage(id: string): Promise<void> {
    return this.request('DELETE', `/pages/${id}`);
  }

  createBlock(
    pageId: string,
    input: { type: BlockType; text?: string; parentId?: string | null; afterBlockId?: string | null; props?: Record<string, unknown> },
  ): Promise<Block> {
    return this.request('POST', `/pages/${pageId}/blocks`, input);
  }
  updateBlock(id: string, input: { text?: string; marks?: Mark[]; props?: Record<string, unknown> }): Promise<Block> {
    return this.request('PATCH', `/blocks/${id}`, input);
  }
  changeBlockType(id: string, type: BlockType): Promise<Block> {
    return this.request('POST', `/blocks/${id}/type`, { type });
  }
  moveBlock(id: string, input: { parentId?: string | null; afterBlockId?: string | null }): Promise<Block> {
    return this.request('POST', `/blocks/${id}/move`, input);
  }
  deleteBlock(id: string): Promise<void> {
    return this.request('DELETE', `/blocks/${id}`);
  }
}
