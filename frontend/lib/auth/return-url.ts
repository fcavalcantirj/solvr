/**
 * Returns `next` only when it is a safe, same-origin relative path: a single
 * leading slash, never a protocol-relative "//host", a "/\\host" backslash trick,
 * or an absolute URL. Anything else — including null/undefined — yields null.
 *
 * This lets a caller honor a ?next= return target after login (task 39, step 3:
 * restore the same room and reading context) without a crafted value redirecting
 * the user off-site once authenticated (open-redirect guard).
 */
export function safeReturnPath(next: string | null | undefined): string | null {
  if (!next) return null;
  if (next[0] !== '/') return null;
  // Reject "//host" and "/\host": browsers can treat both as protocol-relative.
  if (next[1] === '/' || next[1] === '\\') return null;
  return next;
}
