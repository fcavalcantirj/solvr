/**
 * Analytics helpers for Plausible
 * Per SPEC.md Part 19.3 - Privacy-focused analytics
 *
 * IMPORTANT: No PII (Personally Identifiable Information) should be sent to analytics.
 * - Never send email addresses
 * - Never send user IDs
 * - Never send search query content (only length is allowed)
 * - Never send names, addresses, or other identifying information
 */

// List of keys that should be stripped from analytics props (PII)
const PII_KEYS = [
  'email',
  'user_id',
  'userId',
  'user_email',
  'userEmail',
  'username',
  'name',
  'first_name',
  'last_name',
  'firstName',
  'lastName',
  'phone',
  'address',
  'ip',
  'ip_address',
  'query', // Search query content - only query_length is allowed
  'search_query',
  'searchQuery',
  'q', // Common query parameter name
];

/**
 * Strip PII from analytics props
 * @param props - The props to sanitize
 * @returns Sanitized props with PII removed
 */
function stripPII(
  props: Record<string, string | number | boolean>
): Record<string, string | number | boolean> {
  const sanitized: Record<string, string | number | boolean> = {};

  for (const [key, value] of Object.entries(props)) {
    const lowerKey = key.toLowerCase();
    // Check if the key is a PII field
    if (!PII_KEYS.some((piiKey) => lowerKey === piiKey.toLowerCase())) {
      sanitized[key] = value;
    }
  }

  return sanitized;
}

/**
 * Track a custom event with Plausible
 *
 * Per SPEC.md Part 19.3, example events:
 * - Search: { query_length: number, results: number }
 * - Post Created: { type: 'problem'|'question'|'idea', author_type: 'human'|'agent' }
 * - Solution Applied: { post_id: string, time_to_solution: number }
 *
 * @param event - Event name
 * @param props - Event properties (PII will be automatically stripped)
 */
export function trackEvent(event: string, props?: Record<string, string | number | boolean>): void {
  // Check if window.plausible exists (may not be available in SSR or if disabled)
  if (typeof window === 'undefined' || !window.plausible) {
    return;
  }

  // Strip PII from props if provided
  const sanitizedProps = props ? stripPII(props) : undefined;

  // Call Plausible
  if (sanitizedProps && Object.keys(sanitizedProps).length > 0) {
    window.plausible(event, { props: sanitizedProps });
  } else if (!props) {
    window.plausible(event);
  } else {
    // Props were provided but all stripped - still send the event
    window.plausible(event, { props: sanitizedProps });
  }
}

/**
 * Track a pageview (usually automatic, but can be called manually for SPAs)
 */
export function trackPageview(): void {
  trackEvent('pageview');
}
