import { describe, it, expect } from 'vitest';
import { GET } from './route';

describe('sitemap.xml index', () => {
  it('references the canonical posts sitemap, not legacy type-specific sitemaps', async () => {
    const res = await GET();
    const xml = await res.text();

    expect(xml).toContain('https://solvr.dev/sitemap-posts.xml');
    expect(xml).toContain('https://solvr.dev/sitemap-rooms.xml');
    expect(xml).not.toContain('sitemap-problems.xml');
    expect(xml).not.toContain('sitemap-ideas.xml');
  });
});
