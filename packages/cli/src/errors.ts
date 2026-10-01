/**
 * The error of a failed API call: the API's error code, message, details and request id.
 */
export class ApiError extends Error {
  status: number;
  code: string;
  details?: Record<string, unknown>;
  requestId?: string;
  /** The API's error answer as it came ({"error": {...}}); absent when it was not JSON */
  answer?: unknown;

  constructor(
    message: string,
    status: number,
    code: string,
    details?: Record<string, unknown>,
    requestId?: string,
    answer?: unknown
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
    this.requestId = requestId;
    this.answer = answer;
  }

  /** The error as the API answers it: its own answer when there was one. */
  toAnswer(): unknown {
    if (this.answer !== undefined) return this.answer;
    const error: Record<string, unknown> = { code: this.code, message: this.message };
    if (this.requestId) error.request_id = this.requestId;
    if (this.details) error.details = this.details;
    return { error };
  }
}
