/**
 * Tests for robots.txt generation per SPEC.md Part 19.2
 * Tests that robots.txt follows the specification:
 * - Allow: /
 * - Disallow: /admin/
 * - Disallow: /api/
 * - Disallow: /auth/
 * - Sitemap: https://solvr.dev/sitemap.xml
 */

describe('Robots.txt Generation', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
    process.env.NEXT_PUBLIC_APP_URL = 'https://solvr.dev';
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it('exports a valid robots configuration', async () => {
    const robots = await import('../app/robots');
    const result = robots.default();

    expect(result).toBeDefined();
    expect(result.rules).toBeDefined();
  });

  it('allows crawling of root path per SPEC.md Part 19.2', async () => {
    const robots = await import('../app/robots');
    const result = robots.default();

    // Should have at least one rule
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const wildcardRule = rules.find((rule) => rule.userAgent === '*');

    expect(wildcardRule).toBeDefined();
    expect(wildcardRule?.allow).toContain('/');
  });

  it('disallows /admin/ path per SPEC.md Part 19.2', async () => {
    const robots = await import('../app/robots');
    const result = robots.default();

    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const wildcardRule = rules.find((rule) => rule.userAgent === '*');

    expect(wildcardRule?.disallow).toContain('/admin/');
  });

  it('disallows /api/ path per SPEC.md Part 19.2', async () => {
    const robots = await import('../app/robots');
    const result = robots.default();

    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const wildcardRule = rules.find((rule) => rule.userAgent === '*');

    expect(wildcardRule?.disallow).toContain('/api/');
  });

  it('disallows /auth/ path per SPEC.md Part 19.2', async () => {
    const robots = await import('../app/robots');
    const result = robots.default();

    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const wildcardRule = rules.find((rule) => rule.userAgent === '*');

    expect(wildcardRule?.disallow).toContain('/auth/');
  });

  it('includes sitemap URL per SPEC.md Part 19.2', async () => {
    const robots = await import('../app/robots');
    const result = robots.default();

    expect(result.sitemap).toBe('https://solvr.dev/sitemap.xml');
  });

  it('uses base URL from environment variable', async () => {
    process.env.NEXT_PUBLIC_APP_URL = 'https://custom-solvr.io';
    jest.resetModules();

    const robots = await import('../app/robots');
    const result = robots.default();

    expect(result.sitemap).toBe('https://custom-solvr.io/sitemap.xml');
  });

  it('applies rules to all user agents by default', async () => {
    const robots = await import('../app/robots');
    const result = robots.default();

    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const userAgents = rules.map((rule) => rule.userAgent);

    expect(userAgents).toContain('*');
  });
});
