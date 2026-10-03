import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import {
  JsonLd,
  serializeJsonLd,
  blogPostJsonLd,
  roomJsonLd,
  postPageJsonLd,
  websiteJsonLd,
  organizationJsonLd,
  breadcrumbJsonLd,
  agentJsonLd,
} from './json-ld';

describe('blogPostJsonLd', () => {
  const baseBlogPost = {
    title: 'Welcome to Solvr',
    body: '# Introduction\n\nThis is the blog post body with **bold** text.',
    excerpt: 'A short excerpt about the blog post',
    created_at: '2026-02-15T10:00:00Z',
    updated_at: '2026-02-20T15:30:00Z',
    published_at: '2026-02-15T12:00:00Z',
    tags: ['golang', 'postgresql'],
    author: { display_name: 'Alice Developer', type: 'human' as const },
  };

  it('returns BlogPosting schema with correct structure', () => {
    const result = blogPostJsonLd({
      post: baseBlogPost,
      url: 'https://solvr.dev/blog/welcome-to-solvr',
    });

    expect(result['@context']).toBe('https://schema.org');
    expect(result['@type']).toBe('BlogPosting');
    expect(result.headline).toBe('Welcome to Solvr');
    expect(result.datePublished).toBe('2026-02-15T12:00:00Z');
    expect(result.dateModified).toBe('2026-02-20T15:30:00Z');
    expect(result.keywords).toBe('golang, postgresql');
    expect(result.author).toEqual({ '@type': 'Person', name: 'Alice Developer' });
    expect(result.publisher).toEqual({
      '@type': 'Organization',
      name: 'Solvr',
      url: 'https://solvr.dev',
    });
    expect(result.mainEntityOfPage).toEqual({
      '@type': 'WebPage',
      '@id': 'https://solvr.dev/blog/welcome-to-solvr',
    });
  });

  it('uses excerpt as description', () => {
    const result = blogPostJsonLd({
      post: baseBlogPost,
      url: 'https://solvr.dev/blog/test',
    });

    expect(result.description).toBe('A short excerpt about the blog post');
  });

  it('falls back to sanitized body when no excerpt', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, excerpt: undefined },
      url: 'https://solvr.dev/blog/test',
    });

    expect(result.description).not.toContain('#');
    expect(result.description).not.toContain('**');
    expect(result.description).toContain('Introduction');
  });

  it('falls back to default description when no excerpt or body', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, excerpt: undefined, body: '' },
      url: 'https://solvr.dev/blog/test',
    });

    expect(result.description).toBe('A blog post on Solvr');
  });

  it('uses created_at when no published_at', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, published_at: undefined },
      url: 'https://solvr.dev/blog/test',
    });

    expect(result.datePublished).toBe('2026-02-15T10:00:00Z');
  });

  it('handles missing author gracefully', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, author: undefined },
      url: 'https://solvr.dev/blog/test',
    });

    expect(result.author).toBeUndefined();
  });

  it('handles missing tags gracefully', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, tags: undefined },
      url: 'https://solvr.dev/blog/test',
    });

    expect(result.keywords).toBeUndefined();
  });
});


// Task idx 82: structured data describes only what the page shows, truthfully. An agent
// is never labelled a Person, no custom or invented fields, canonical URLs, real dates.

describe('roomJsonLd', () => {
  const room = {
    display_name: 'Kestrel Room',
    message_count: 42,
    created_at: '2026-01-01T00:00:00Z',
    last_active_at: '2026-04-01T00:00:00Z',
  };
  const url = 'https://solvr.dev/rooms/kestrel-room';

  it('an agent-led room is a WebPage about a CreativeWork, never a Person', () => {
    const result = roomJsonLd({
      room, url, description: 'Plan and ship the kestrel tracker',
      firstMessage: { author_type: 'agent', agent_name: 'seed-planner', content: 'Plan step one' },
    });
    expect(result['@type']).toBe('WebPage');
    const work = result.mainEntity as Record<string, unknown>;
    expect(work['@type']).toBe('CreativeWork');
    expect(work.headline).toBe('Kestrel Room');
    expect(work.datePublished).toBe('2026-01-01T00:00:00Z');
    expect(work.dateModified).toBe('2026-04-01T00:00:00Z');
    expect((work.interactionStatistic as { userInteractionCount: number }).userInteractionCount).toBe(42);
    expect(JSON.stringify(result)).not.toContain('Person');
  });

  it('a human-led room is a DiscussionForumPosting with the human as author', () => {
    const result = roomJsonLd({
      room, url, description: 'Plan',
      firstMessage: { author_type: 'human', agent_name: 'reviewer-human', author_id: 'u-1', content: 'Kick off the kestrel plan' },
    });
    expect(result['@type']).toBe('DiscussionForumPosting');
    expect(result.author).toEqual({ '@type': 'Person', name: 'reviewer-human', url: 'https://solvr.dev/users/u-1' });
    expect(result.text).toBe('Kick off the kestrel plan');
    expect(result.headline).toBe('Kestrel Room');
    expect(result.url).toBe(url);
  });

  it('carries no custom or inaccurate fields and invents no description', () => {
    const result = roomJsonLd({ room, url, description: undefined, firstMessage: null });
    const text = JSON.stringify(result);
    expect(text).not.toContain('additionalProperty');
    expect(text).not.toContain('machineGeneratedContent');
    expect(text).not.toContain('SoftwareSourceCode');
    expect(text).not.toContain('A2A room on Solvr');
  });
});

describe('postPageJsonLd', () => {
  const post = {
    title: 'Hand a plan to an executor',
    description: 'The executor must pick the plan up.',
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-02T00:00:00Z',
    reply_count: 3,
  };
  const url = 'https://solvr.dev/posts/p1';

  it('a human post is a DiscussionForumPosting with a Person author and its profile', () => {
    const result = postPageJsonLd({
      post: { ...post, author: { id: 'u-9', type: 'human', display_name: 'Dev Nine' } },
      url, headline: 'Hand a plan to an executor',
    });
    expect(result['@type']).toBe('DiscussionForumPosting');
    expect(result.author).toEqual({ '@type': 'Person', name: 'Dev Nine', url: 'https://solvr.dev/users/u-9' });
    expect(result.text).toBe('The executor must pick the plan up.');
    expect(result.datePublished).toBe('2026-09-01T00:00:00Z');
    expect(result.dateModified).toBe('2026-09-02T00:00:00Z');
    expect(result.url).toBe(url);
  });

  it('an agent post is a WebPage about a CreativeWork and names no Person', () => {
    const result = postPageJsonLd({
      post: { ...post, author: { id: 'a-1', type: 'agent', display_name: 'Helper Bot' } },
      url, headline: 'Hand a plan to an executor — Helper Bot',
    });
    expect(result['@type']).toBe('WebPage');
    expect((result.mainEntity as Record<string, unknown>).headline).toBe('Hand a plan to an executor — Helper Bot');
    expect(JSON.stringify(result)).not.toContain('Person');
  });
});

describe('site-wide schema', () => {
  it('describes the website and the organization at their canonical URL', () => {
    expect(websiteJsonLd()).toMatchObject({ '@type': 'WebSite', name: 'Solvr', url: 'https://solvr.dev/' });
    expect(organizationJsonLd()).toMatchObject({ '@type': 'Organization', name: 'Solvr', url: 'https://solvr.dev/' });
  });

  it('builds breadcrumbs with absolute canonical URLs in order', () => {
    expect(breadcrumbJsonLd([{ name: 'Rooms', path: '/rooms' }, { name: 'Kestrel Room', path: '/rooms/kestrel-room' }])).toEqual({
      '@context': 'https://schema.org',
      '@type': 'BreadcrumbList',
      itemListElement: [
        { '@type': 'ListItem', position: 1, name: 'Solvr', item: 'https://solvr.dev/' },
        { '@type': 'ListItem', position: 2, name: 'Rooms', item: 'https://solvr.dev/rooms' },
        { '@type': 'ListItem', position: 3, name: 'Kestrel Room', item: 'https://solvr.dev/rooms/kestrel-room' },
      ],
    });
  });
});

describe('blogPostJsonLd authors', () => {
  it('never labels an agent author a Person', () => {
    const result = blogPostJsonLd({
      post: {
        title: 'Agent notes', body: 'body', created_at: '2026-02-15T10:00:00Z', updated_at: '2026-02-15T10:00:00Z',
        author: { display_name: 'Helper Bot', type: 'agent' },
      },
      url: 'https://solvr.dev/blog/agent-notes',
    });
    expect(result.author).toBeUndefined();
  });
});

describe('JSON-LD escaping', () => {
  const hostile = { '@type': 'WebPage', name: '</script><script>alert(1)</script> & \u2028 <!--' };

  it('serializes so no content can close the script element, and round-trips', () => {
    const out = serializeJsonLd(hostile);
    expect(out).not.toMatch(/<\/?script/i);
    expect(out).not.toContain('<!--');
    expect(JSON.parse(out)).toEqual(hostile);
  });

  it('renders the escaped form inside the script tag', () => {
    const { container } = render(<JsonLd data={hostile} />);
    const html = container.innerHTML;
    expect(html.match(/<script/gi)).toHaveLength(1);
    expect(html.match(/<\/script>/gi)).toHaveLength(1);
  });
});

describe('agentJsonLd', () => {
  it('describes the agent without passing its model off as an operating system', () => {
    const result = agentJsonLd({ agent: { display_name: 'Helper Bot', model: 'some-model-1' }, url: 'https://solvr.dev/agents/a-1' });
    expect(result['@type']).toBe('SoftwareApplication');
    expect(JSON.stringify(result)).not.toContain('operatingSystem');
  });
});
