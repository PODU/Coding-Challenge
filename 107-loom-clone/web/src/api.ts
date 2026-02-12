import type {
  CompleteUploadResponse,
  CreateUploadResponse,
  ListRecordingsResponse,
  PublicRecording,
  Recording,
  UploadStatusResponse,
} from '../../shared/types';

const API = '/api';

async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { error?: { message?: string } };
      if (body.error?.message) {
        message = body.error.message;
      }
    } catch {
      // Not a JSON error body; keep the status text.
    }
    throw new Error(message);
  }
  return (await res.json()) as T;
}

export async function createUpload(input: {
  filename: string;
  mimeType: string;
  totalBytes?: number;
}): Promise<CreateUploadResponse> {
  const res = await fetch(`${API}/uploads`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  return json<CreateUploadResponse>(res);
}

export async function putChunk(uploadId: string, index: number, chunk: Blob): Promise<void> {
  const res = await fetch(`${API}/uploads/${uploadId}/chunk?index=${index}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/octet-stream' },
    body: chunk,
  });
  if (!res.ok) {
    throw new Error(`Chunk ${index} failed: HTTP ${res.status}`);
  }
}

export async function getUploadStatus(uploadId: string): Promise<UploadStatusResponse> {
  return json<UploadStatusResponse>(await fetch(`${API}/uploads/${uploadId}`));
}

export async function completeUpload(
  uploadId: string,
  input: { title?: string; durationMs?: number },
): Promise<CompleteUploadResponse> {
  const res = await fetch(`${API}/uploads/${uploadId}/complete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  return json<CompleteUploadResponse>(res);
}

export async function listRecordings(limit?: number, cursor?: string): Promise<ListRecordingsResponse> {
  const params = new URLSearchParams();
  if (limit !== undefined) params.set('limit', String(limit));
  if (cursor !== undefined) params.set('cursor', cursor);
  const qs = params.toString();
  return json<ListRecordingsResponse>(await fetch(`${API}/recordings${qs ? `?${qs}` : ''}`));
}

export async function getRecording(id: string): Promise<Recording> {
  return json<Recording>(await fetch(`${API}/recordings/${id}`));
}

export async function deleteRecording(id: string): Promise<void> {
  const res = await fetch(`${API}/recordings/${id}`, { method: 'DELETE' });
  if (!res.ok && res.status !== 404) {
    throw new Error(`Delete failed: HTTP ${res.status}`);
  }
}

export async function resolveShare(shareId: string): Promise<PublicRecording> {
  return json<PublicRecording>(await fetch(`${API}/share/${shareId}`));
}

export function shareStreamUrl(shareId: string): string {
  return `${API}/share/${shareId}/stream`;
}
