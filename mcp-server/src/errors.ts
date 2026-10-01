/**
 * The error of a failed API call: its HTTP status, the API's error code and request id.
 * The message keeps the form "API request failed: <status> <status text>: CODE: message".
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly requestId?: string;

  constructor(message: string, status: number, code?: string, requestId?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}
