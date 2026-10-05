import { describe, it, expect } from 'vitest';
import { cleanParams, redact } from './analytics-redact';

// Nothing a person could be identified by, and nothing that opens an account, leaves the
// browser inside an event parameter: every string is stripped, then cut (SPEC.md 27.7).

const JWT =
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c';

describe('redact', () => {
  it('strips an e-mail address', () => {
    expect(redact('error for felipe.cavalcanti+test@example.com in room')).toBe('error for [redacted] in room');
  });

  it('strips a Solvr key, agent or user', () => {
    expect(redact('key solvr_sk_live_AbC-123_xyz failed')).toBe('key [redacted] failed');
    expect(redact('solvr_9f8e7d6c5b4a')).toBe('[redacted]');
  });

  it('strips a bearer token together with the word', () => {
    expect(redact('curl -H "Authorization: Bearer abc.DEF-123_456" /v1/me')).toBe(
      'curl -H "Authorization: [redacted]" /v1/me',
    );
    expect(redact('bearer   xyz987')).toBe('[redacted]');
  });

  it('strips a string that looks like a JWT', () => {
    expect(redact(`token=${JWT}&next=/`)).toBe('token=[redacted]&next=/');
  });

  it('strips a JWT that lost its signature', () => {
    expect(redact('eyJhbGciOiJub25lIn0.eyJzdWIiOiIxIn0.')).toBe('[redacted]');
    expect(redact('eyJhbGciOiJub25lIn0.eyJzdWIiOiIxIn0')).toBe('[redacted]');
  });

  it('strips every secret in one string', () => {
    expect(redact(`a@b.co solvr_abc ${JWT}`)).toBe('[redacted] [redacted] [redacted]');
  });

  it('leaves ordinary text and ids alone', () => {
    for (const plain of ['copy_prompt', 'plan-and-build', 'postgres connection pool', 'SOLVR_ wordmark', 'a.b.c', 'v1.3.14']) {
      expect(redact(plain)).toBe(plain);
    }
  });

  it('cuts to 100 characters, after stripping', () => {
    expect(redact('x'.repeat(250))).toBe('x'.repeat(100));
    // The address straddles the cut: cutting first would leave half of it behind.
    const straddling = `${'y'.repeat(95)} someone@example.com`;
    const out = redact(straddling);
    expect(out.length).toBeLessThanOrEqual(100);
    expect(out).not.toContain('someone');
    expect(out).not.toContain('@');
  });

  it('never splits a character in two when it cuts', () => {
    const out = redact('😀'.repeat(150));
    expect(Array.from(out)).toHaveLength(100);
    expect(out.endsWith('😀')).toBe(true);
  });
});

describe('cleanParams', () => {
  it('answers an empty object for no parameters', () => {
    expect(cleanParams(undefined)).toEqual({});
    expect(cleanParams({})).toEqual({});
  });

  it('drops undefined values and keeps numbers and booleans as they are', () => {
    expect(cleanParams({ item: 'rooms', location: undefined, results: 0, page: 3 })).toEqual({
      item: 'rooms',
      results: 0,
      page: 3,
    });
  });

  it('strips and cuts every string parameter', () => {
    expect(cleanParams({ search_term: `who is a@b.co ${'z'.repeat(200)}`, item: 'solvr_sk_abc' })).toEqual({
      search_term: `who is [redacted] ${'z'.repeat(82)}`,
      item: '[redacted]',
    });
  });

  it('drops a parameter the closed set does not name, and a value that is not a plain one', () => {
    const sneaky = { item: 'rooms', email: 'a@b.co', surface: { nested: true }, role: null } as unknown as Parameters<typeof cleanParams>[0];
    expect(cleanParams(sneaky)).toEqual({ item: 'rooms' });
  });
});
