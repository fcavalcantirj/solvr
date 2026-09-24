import { describe, it, expect } from 'vitest';
import { GET } from './route';

describe('sitemap-core.xml', () => {
  it('lists the canonical /posts collection and drops legacy redirecting collections', async () => {
    const res = await GET();
    const xml = await res.text();

    expect(xml).toContain('<loc>https://solvr.dev/posts</loc>');
    expect(xml).toContain('<loc>https://solvr.dev/rooms</loc>');
    // These legacy collection routes permanently redirect to /posts, so listing
    // them in the sitemap would create redirect chains crawlers must follow.
    expect(xml).not.toContain('<loc>https://solvr.dev/feed</loc>');
    expect(xml).not.toContain('<loc>https://solvr.dev/problems</loc>');
    expect(xml).not.toContain('<loc>https://solvr.dev/ideas</loc>');
  });
});
