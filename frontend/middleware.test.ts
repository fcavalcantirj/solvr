import { describe, it, expect, vi } from 'vitest';
import { NextRequest } from 'next/server';
import { middleware, canonicalPath, config } from './middleware';

describe('canonicalPath (legacy → canonical route mapping)', () => {
  it('collapses legacy collection routes to /posts', () => {
    expect(canonicalPath('/feed')).toBe('/posts');
    expect(canonicalPath('/problems')).toBe('/posts');
    expect(canonicalPath('/ideas')).toBe('/posts');
    expect(canonicalPath('/questions')).toBe('/posts');
  });

  it('maps legacy detail routes to /posts/{same-id}', () => {
    expect(canonicalPath('/problems/abc-123')).toBe('/posts/abc-123');
    expect(canonicalPath('/ideas/idea-9')).toBe('/posts/idea-9');
    expect(canonicalPath('/questions/q-7')).toBe('/posts/q-7');
  });

  it('preserves the new-composer and edit sub-paths', () => {
    expect(canonicalPath('/problems/new')).toBe('/posts/new');
    expect(canonicalPath('/ideas/new')).toBe('/posts/new');
    expect(canonicalPath('/problems/abc-123/edit')).toBe('/posts/abc-123/edit');
  });

  it('never rewrites a canonical /posts path (no redirect loop)', () => {
    expect(canonicalPath('/posts')).toBeNull();
    expect(canonicalPath('/posts/abc-123')).toBeNull();
    expect(canonicalPath('/posts/new')).toBeNull();
  });

  it('leaves unrelated routes untouched', () => {
    expect(canonicalPath('/')).toBeNull();
    expect(canonicalPath('/rooms/some-room')).toBeNull();
    expect(canonicalPath('/connect')).toBeNull();
  });
});

describe('middleware', () => {
  it('permanently (308) redirects a legacy detail URL to its canonical destination and preserves the query string', () => {
    const res = middleware(new NextRequest('http://localhost/problems/abc-123?sort=top&tag=go'));
    expect(res.status).toBe(308);
    const loc = new URL(res.headers.get('location') as string);
    expect(loc.pathname).toBe('/posts/abc-123');
    expect(loc.search).toBe('?sort=top&tag=go');
  });

  it('permanently (308) redirects a legacy collection URL to /posts', () => {
    const res = middleware(new NextRequest('http://localhost/feed'));
    expect(res.status).toBe(308);
    expect(new URL(res.headers.get('location') as string).pathname).toBe('/posts');
  });

  it('does not redirect the canonical /posts route', () => {
    const res = middleware(new NextRequest('http://localhost/posts'));
    expect(res.status).not.toBe(308);
  });

  it('still blocks the guarded /adfa path with a 404', () => {
    const res = middleware(new NextRequest('http://localhost/adfa'));
    expect(res.status).toBe(404);
  });
});

describe('the old /new composer (idx 52: the web client creates canonical posts)', () => {
  it('maps /new to the canonical composer and leaves look-alike routes alone', () => {
    expect(canonicalPath('/new')).toBe('/posts/new');
    expect(canonicalPath('/news')).toBeNull();
    expect(canonicalPath('/newsletter')).toBeNull();
    expect(canonicalPath('/posts/new')).toBeNull();
  });

  it('permanently (308) redirects /new?type=problem to /posts/new', () => {
    const res = middleware(new NextRequest('http://localhost/new?type=problem'));
    expect(res.status).toBe(308);
    expect(new URL(res.headers.get('location') as string).pathname).toBe('/posts/new');
  });

  it('runs the middleware on /new', () => {
    expect(config.matcher).toContain('/new');
  });
});

// Task idx 83: the legacy redirects are permanent and outlive the API's compatibility
// sunset. They are decided from the path alone, so they never ask the API: a retired or
// unreachable API cannot turn a legacy URL into an error or a redirect chain.
describe('legacy redirects need no API', () => {
  it('answers every legacy URL with one 308 to its exact canonical post, without fetching', () => {
    const fetchSpy = vi.fn(() => {
      throw new Error('the API was called');
    });
    vi.stubGlobal('fetch', fetchSpy);
    try {
      for (const [legacy, canonical] of [
        ['/problems/2f0c6a3e-0000-4000-8000-000000000001', '/posts/2f0c6a3e-0000-4000-8000-000000000001'],
        ['/ideas/2f0c6a3e-0000-4000-8000-000000000002', '/posts/2f0c6a3e-0000-4000-8000-000000000002'],
        ['/questions/2f0c6a3e-0000-4000-8000-000000000003', '/posts/2f0c6a3e-0000-4000-8000-000000000003'],
      ]) {
        const res = middleware(new NextRequest(`https://solvr.dev${legacy}`));
        expect(res.status).toBe(308);
        expect(new URL(res.headers.get('location')!).pathname).toBe(canonical);
        expect(new URL(res.headers.get('location')!).pathname).not.toBe('/');
      }
      expect(fetchSpy).not.toHaveBeenCalled();
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
