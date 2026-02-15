import { describe, it, expect } from 'vitest';
import robots from './robots';

describe('robots', () => {
  it('returns correct rules with sitemap URL', () => {
    const result = robots();

    expect(result.sitemap).toBe('https://solvr.dev/sitemap.xml');
  });

  it('allows all user-agents to crawl /', () => {
    const result = robots();

    expect(result.rules).toBeDefined();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const allRule = rules.find((r) => r.userAgent === '*');
    expect(allRule).toBeDefined();
    expect(allRule!.allow).toBe('/');
  });

  it('disallows settings, auth, login, join, new, and admin paths', () => {
    const result = robots();

    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const allRule = rules.find((r) => r.userAgent === '*');
    expect(allRule).toBeDefined();

    const disallowed = allRule!.disallow as string[];
    expect(disallowed).toContain('/settings/');
    expect(disallowed).toContain('/auth/');
    expect(disallowed).toContain('/login');
    expect(disallowed).toContain('/join');
    expect(disallowed).toContain('/new');
    expect(disallowed).toContain('/admin/');
  });

  it('sets host to https://solvr.dev', () => {
    const result = robots();

    expect(result.host).toBe('https://solvr.dev');
  });
});
