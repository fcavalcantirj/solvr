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

  // Task idx 80: settings, auth, login, join, admin and dashboard pages are kept out
  // of search by their own noindex (lib/seo/route-policy.ts), which a crawler can
  // only obey if robots.txt lets it fetch them.
  it('disallows no site path for regular crawlers, so noindex pages can be read', () => {
    const result = robots();

    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const allRule = rules.find((r) => r.userAgent === '*');
    expect(allRule).toBeDefined();
    expect(allRule!.disallow).toBeUndefined();
  });

  it('blocks SEO crawler bots to save crawl budget', () => {
    const result = robots();

    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const blockedBots = ['semrushbot', 'ahrefsbot', 'MJ12bot'];
    for (const bot of blockedBots) {
      const rule = rules.find((r) => r.userAgent === bot);
      expect(rule, `expected rule for ${bot}`).toBeDefined();
      expect(rule!.disallow).toBe('/');
    }
  });

  it('sets host to https://solvr.dev', () => {
    const result = robots();

    expect(result.host).toBe('https://solvr.dev');
  });
});
