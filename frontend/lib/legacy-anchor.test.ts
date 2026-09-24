import { describe, it, expect } from 'vitest';
import { extractLegacyId, resolveLegacyAnchor } from '@/lib/legacy-anchor';

describe('extractLegacyId', () => {
  it('strips known legacy contribution prefixes', () => {
    expect(extractLegacyId('#approach-abc')).toBe('abc');
    expect(extractLegacyId('#answer-def')).toBe('def');
    expect(extractLegacyId('#response-ghi')).toBe('ghi');
    expect(extractLegacyId('#comment-jkl')).toBe('jkl');
  });

  it('accepts a bare id anchor with no known prefix', () => {
    expect(extractLegacyId('#legacy-uuid-123')).toBe('legacy-uuid-123');
  });

  it('returns null for an empty or bare-hash value', () => {
    expect(extractLegacyId('')).toBeNull();
    expect(extractLegacyId('#')).toBeNull();
  });
});

describe('resolveLegacyAnchor', () => {
  const replies = [
    { id: 'reply-1', legacy_id: 'legacy-A' },
    { id: 'reply-2', legacy_id: 'legacy-B' },
    { id: 'reply-3' },
  ];

  it('maps a legacy contribution anchor to its canonical reply id via the migration mapping', () => {
    expect(resolveLegacyAnchor('#approach-legacy-A', replies)).toBe('reply-1');
    expect(resolveLegacyAnchor('#answer-legacy-B', replies)).toBe('reply-2');
  });

  it('returns null when no reply carries that legacy id', () => {
    expect(resolveLegacyAnchor('#approach-unknown', replies)).toBeNull();
  });

  it('returns null for an empty hash', () => {
    expect(resolveLegacyAnchor('', replies)).toBeNull();
  });
});
