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
  userJsonLd,
} from './json-ld';

describe('blogPostJsonLd', () => {
  const baseBlogPost = {
    title: 'Welcome to Solvr',
    created_at: '2026-02-15T10:00:00Z',
    updated_at: '2026-02-20T15:30:00Z',
    published_at: '2026-02-15T12:00:00Z',
    tags: ['golang', 'postgresql'],
    author: { id: 'u-1', display_name: 'Alice Developer', type: 'human' as const },
  };
  const description = 'What the API serves as the post description';

  it('returns BlogPosting schema with correct structure', () => {
    const result = blogPostJsonLd({
      post: baseBlogPost,
      url: 'https://solvr.dev/blog/welcome-to-solvr',
      description,
    });

    expect(result['@context']).toBe('https://schema.org');
    expect(result['@type']).toBe('BlogPosting');
    expect(result.headline).toBe('Welcome to Solvr');
    expect(result.datePublished).toBe('2026-02-15T12:00:00Z');
    expect(result.dateModified).toBe('2026-02-20T15:30:00Z');
    expect(result.keywords).toBe('golang, postgresql');
    expect(result.author).toEqual({ '@type': 'Person', name: 'Alice Developer', url: 'https://solvr.dev/users/u-1' });
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

  // SPEC.md 27.1/27.3: the description is the API's (the served meta_description), the one
  // the page's meta description and link preview state. The page derives none.
  it("states the API's description", () => {
    const result = blogPostJsonLd({ post: baseBlogPost, url: 'https://solvr.dev/blog/test', description });
    expect(result.description).toBe(description);
  });

  it('uses created_at when no published_at', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, published_at: undefined },
      url: 'https://solvr.dev/blog/test',
      description,
    });

    expect(result.datePublished).toBe('2026-02-15T10:00:00Z');
  });

  it('handles missing author gracefully', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, author: undefined },
      url: 'https://solvr.dev/blog/test',
      description,
    });

    expect(result.author).toBeUndefined();
  });

  // A post whose author's account is gone is served with no author name (measured on the
  // local stack): it names no author rather than a Person without a name.
  it('names no author when the API serves no name for one', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, author: { id: 'gone-1', display_name: '', type: 'human' } },
      url: 'https://solvr.dev/blog/test',
      description,
    });
    expect(result.author).toBeUndefined();
  });

  it('handles missing tags gracefully', () => {
    const result = blogPostJsonLd({
      post: { ...baseBlogPost, tags: undefined },
      url: 'https://solvr.dev/blog/test',
      description,
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
  // The recon (2026-10-04) found no author on 32 of 32 blog posts: every one was an agent's,
  // and an agent was given none. It is named as the API names it, as a plain Thing.
  it('names an agent author as a Thing with its profile, never a Person', () => {
    const result = blogPostJsonLd({
      post: {
        title: 'Agent notes', created_at: '2026-02-15T10:00:00Z', updated_at: '2026-02-15T10:00:00Z',
        author: { id: 'helper_bot', display_name: 'Helper Bot', type: 'agent' },
      },
      url: 'https://solvr.dev/blog/agent-notes',
      description: 'Notes',
    });
    expect(result.author).toEqual({ '@type': 'Thing', name: 'Helper Bot', url: 'https://solvr.dev/agents/helper_bot', identifier: 'helper_bot' });
    expect(JSON.stringify(result)).not.toContain('Person');
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

// SPEC.md 27.3: a profile page is a ProfilePage. An agent is a plain Thing (a
// SoftwareApplication promises a price and ratings the page does not show); a person is the
// Person under their public name. The description is the API's /seo description.
describe('agentJsonLd', () => {
  it('is a ProfilePage about a plain Thing with name, description, url and identifier', () => {
    const result = agentJsonLd({
      agent: { id: 'helper_bot', display_name: 'Helper Bot' },
      url: 'https://solvr.dev/agents/helper_bot',
      description: 'Helper Bot, an AI agent on Solvr: 2 posts.',
    });
    expect(result).toEqual({
      '@context': 'https://schema.org',
      '@type': 'ProfilePage',
      url: 'https://solvr.dev/agents/helper_bot',
      mainEntity: {
        '@type': 'Thing',
        name: 'Helper Bot',
        description: 'Helper Bot, an AI agent on Solvr: 2 posts.',
        url: 'https://solvr.dev/agents/helper_bot',
        identifier: 'helper_bot',
      },
      publisher: { '@type': 'Organization', name: 'Solvr', url: 'https://solvr.dev' },
    });
    expect(JSON.stringify(result)).not.toContain('SoftwareApplication');
  });

  it('names an agent without a display name by its id and states no description it was not given', () => {
    const result = agentJsonLd({ agent: { id: 'helper_bot', display_name: '' }, url: 'https://solvr.dev/agents/helper_bot' });
    expect(result.mainEntity).toEqual({ '@type': 'Thing', name: 'helper_bot', url: 'https://solvr.dev/agents/helper_bot', identifier: 'helper_bot' });
  });
});

describe('userJsonLd', () => {
  it('is a ProfilePage about the Person under the public name the API serves', () => {
    const result = userJsonLd({
      user: { display_name: 'Ana Lima', username: 'ana' },
      url: 'https://solvr.dev/users/u-1',
      description: 'Ana Lima on Solvr: 1 post.',
    });
    expect(result['@type']).toBe('ProfilePage');
    expect(result.mainEntity).toEqual({
      '@type': 'Person',
      name: 'Ana Lima',
      alternateName: 'ana',
      description: 'Ana Lima on Solvr: 1 post.',
      url: 'https://solvr.dev/users/u-1',
    });
  });

  // The recon found 7 ProfilePage blocks with no name: a person without a display name.
  it('names a person without a display name by their username', () => {
    const result = userJsonLd({ user: { display_name: '', username: 'ana' }, url: 'https://solvr.dev/users/u-1' });
    expect((result.mainEntity as { name: string }).name).toBe('ana');
  });
});
