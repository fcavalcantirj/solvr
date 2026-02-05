/**
 * Custom error class for API errors that includes HTTP status code.
 */
export class APIError extends Error {
  public readonly statusCode: number;

  constructor(message: string, statusCode: number) {
    super(message);
    this.name = 'APIError';
    this.statusCode = statusCode;
  }
}

/**
 * Check if an error is a 404 Not Found error.
 */
export function isNotFoundError(error: unknown): boolean {
  return error instanceof APIError && error.statusCode === 404;
}

/**
 * Check if an error is a server error (5xx).
 */
export function isServerError(error: unknown): boolean {
  return error instanceof APIError && error.statusCode >= 500 && error.statusCode < 600;
}

/**
 * Get a user-friendly error message based on the error type.
 */
export function getErrorMessage(error: unknown): string {
  if (isNotFoundError(error)) {
    return 'The requested resource was not found.';
  }
  if (isServerError(error)) {
    return 'A server error occurred. Please try again later.';
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'An unexpected error occurred.';
}
