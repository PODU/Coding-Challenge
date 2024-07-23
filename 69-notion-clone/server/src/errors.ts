/** Error envelope codes. */
export type ErrorCode =
  | 'VALIDATION_ERROR'
  | 'UNAUTHENTICATED'
  | 'FORBIDDEN'
  | 'NOT_FOUND'
  | 'CONFLICT'
  | 'INTERNAL';

const STATUS_BY_CODE: Record<ErrorCode, number> = {
  VALIDATION_ERROR: 400,
  UNAUTHENTICATED: 401,
  FORBIDDEN: 403,
  NOT_FOUND: 404,
  CONFLICT: 409,
  INTERNAL: 500,
};

/** Domain/HTTP error carrying a stable code mapped to an HTTP status. */
export class AppError extends Error {
  readonly code: ErrorCode;
  readonly status: number;
  readonly details: unknown[];

  constructor(code: ErrorCode, message: string, details: unknown[] = []) {
    super(message);
    this.name = 'AppError';
    this.code = code;
    this.status = STATUS_BY_CODE[code];
    this.details = details;
  }

  static validation(message: string, details: unknown[] = []): AppError {
    return new AppError('VALIDATION_ERROR', message, details);
  }
  static unauthenticated(message = 'Authentication required'): AppError {
    return new AppError('UNAUTHENTICATED', message);
  }
  static notFound(message = 'Resource not found'): AppError {
    return new AppError('NOT_FOUND', message);
  }
  static conflict(message: string): AppError {
    return new AppError('CONFLICT', message);
  }
}
