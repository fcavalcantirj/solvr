/**
 * API Client for Solvr
 * Per SPEC.md Part 5 API Specification
 */

const AUTH_TOKEN_KEY = 'solvr_auth_token';

/**
 * Error codes per SPEC.md Part 5.4
 */
export type ApiErrorCode =
  | 'UNAUTHORIZED'
  | 'FORBIDDEN'
  | 'NOT_FOUND'
  | 'VALIDATION_ERROR'
  | 'RATE_LIMITED'
  | 'DUPLICATE_CONTENT'
  | 'CONTENT_TOO_SHORT'
  | 'INTERNAL_ERROR';

/**
 * Custom error class for API errors
 */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: ApiErrorCode | string,
    message: string,
    public readonly details?: Record<string, unknown>
  ) {
    super(message);
    this.name = 'ApiError';
    // Maintains proper prototype chain for instanceof checks
    Object.setPrototypeOf(this, ApiError.prototype);
  }
}

/**
 * Response envelope per SPEC.md Part 5.3
 */
interface ApiResponse<T> {
  data: T;
  meta?: {
    timestamp?: string;
    total?: number;
    page?: number;
    per_page?: number;
    has_more?: boolean;
    [key: string]: unknown;
  };
}

/**
 * Error response per SPEC.md Part 5.3
 */
interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
}

/**
 * Request options
 */
interface RequestOptions {
  includeMetadata?: boolean;
}

/**
 * Get the API base URL from environment or default
 */
function getBaseUrl(): string {
  return process.env.NEXT_PUBLIC_API_URL || '/api';
}

/**
 * Get auth token from localStorage
 */
export function getAuthToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(AUTH_TOKEN_KEY);
}

/**
 * Set auth token in localStorage
 */
export function setAuthToken(token: string): void {
  if (typeof window === 'undefined') return;
  localStorage.setItem(AUTH_TOKEN_KEY, token);
}

/**
 * Clear auth token from localStorage
 */
export function clearAuthToken(): void {
  if (typeof window === 'undefined') return;
  localStorage.removeItem(AUTH_TOKEN_KEY);
}

/**
 * Build headers for API requests
 */
function buildHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  const token = getAuthToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  return headers;
}

/**
 * Build URL with query parameters
 */
function buildUrl(path: string, params?: Record<string, string>): string {
  const baseUrl = getBaseUrl();
  const url = new URL(path, baseUrl.startsWith('http') ? baseUrl : `http://localhost${baseUrl}`);

  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      url.searchParams.append(key, value);
    });
  }

  // Return full URL for absolute base URLs, otherwise just path + search
  if (baseUrl.startsWith('http')) {
    return url.toString();
  }

  return `${baseUrl}${path}${url.search}`;
}

/**
 * Parse API response and extract data
 */
async function parseResponse<T>(response: Response, options?: RequestOptions): Promise<T> {
  if (!response.ok) {
    let errorData: ApiErrorResponse | null = null;

    try {
      errorData = (await response.json()) as ApiErrorResponse;
    } catch {
      // JSON parsing failed, create generic error
      throw new ApiError(response.status, 'INTERNAL_ERROR', `HTTP ${response.status} error`);
    }

    throw new ApiError(
      response.status,
      errorData.error.code,
      errorData.error.message,
      errorData.error.details
    );
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  const json = (await response.json()) as ApiResponse<T>;

  // Return full response if metadata requested
  if (options?.includeMetadata) {
    return json as unknown as T;
  }

  // Unwrap data envelope
  return json.data;
}

/**
 * Make a GET request
 */
async function get<T>(
  path: string,
  params?: Record<string, string>,
  options?: RequestOptions
): Promise<T> {
  const url = buildUrl(path, params);
  const response = await fetch(url, {
    method: 'GET',
    headers: buildHeaders(),
  });

  return parseResponse<T>(response, options);
}

/**
 * Make a POST request
 */
async function post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T> {
  const url = buildUrl(path);
  const response = await fetch(url, {
    method: 'POST',
    headers: buildHeaders(),
    body: body ? JSON.stringify(body) : undefined,
  });

  return parseResponse<T>(response, options);
}

/**
 * Make a PATCH request
 */
async function patch<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T> {
  const url = buildUrl(path);
  const response = await fetch(url, {
    method: 'PATCH',
    headers: buildHeaders(),
    body: body ? JSON.stringify(body) : undefined,
  });

  return parseResponse<T>(response, options);
}

/**
 * Make a DELETE request
 */
async function del<T>(path: string, options?: RequestOptions): Promise<T> {
  const url = buildUrl(path);
  const response = await fetch(url, {
    method: 'DELETE',
    headers: buildHeaders(),
  });

  return parseResponse<T>(response, options);
}

/**
 * API client object with all methods
 */
export const api = {
  get,
  post,
  patch,
  delete: del,
};

export default api;
