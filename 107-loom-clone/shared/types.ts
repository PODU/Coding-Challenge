// Types shared by the server and the browser client. Keep this file type-only:
// the server compiles it as CommonJS and the web build as ESM.

export interface Recording {
  /** Never exposed on share endpoints. */
  id: string;
  /** Token used in /v/:shareId. */
  shareId: string;
  /** Defaults to a timestamp. */
  title: string;
  /** Storage key of the blob. */
  filename: string;
  /** e.g. video/webm;codecs=vp9,opus */
  mimeType: string;
  sizeBytes: number;
  /** As reported by the recorder; may be 0. */
  durationMs: number;
  thumbnailKey?: string;
  /** ISO-8601. */
  createdAt: string;
}

/** What /share/:shareId returns. */
export interface PublicRecording {
  shareId: string;
  title: string;
  mimeType: string;
  sizeBytes: number;
  durationMs: number;
  createdAt: string;
}

export interface UploadSession {
  uploadId: string;
  filename: string;
  mimeType: string;
  totalBytes?: number;
  /** Contiguous indices received so far. */
  receivedChunks: number[];
  createdAt: string;
}

export interface CreateUploadRequest {
  filename: string;
  mimeType: string;
  totalBytes?: number;
}

export interface CreateUploadResponse {
  uploadId: string;
}

export interface UploadStatusResponse {
  receivedChunks: number[];
  nextExpectedIndex: number;
}

export interface CompleteUploadRequest {
  title?: string;
  durationMs?: number;
}

export interface CompleteUploadResponse extends Recording {
  shareUrl: string;
}

export interface ListRecordingsResponse {
  items: Recording[];
  nextCursor?: string;
}

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
  };
}
