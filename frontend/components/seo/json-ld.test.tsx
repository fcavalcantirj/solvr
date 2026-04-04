import { describe, it, expect } from 'vitest';
import { blogPostJsonLd, roomJsonLd } from './json-ld';

describe('blogPostJsonLd', () => {
  const baseBlogPost = {
    title: 'Welcome to Solvr',
    body: '# Introduction\n\nThis is the blog post body with **bold** text.',
    excerpt: 'A short excerpt about the blog post',
    created_at: '2026-02-15T10:00:00Z',
    updated_at: '2026-02-20T15:30:00Z',
    published_at: '2026-02-15T12:00:00Z',
    tags: ['golang', 'postgresql'],
    author: { display_name: 'Alice Developer' },
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

describe('roomJsonLd', () => {
  const baseRoom = {
    display_name: 'Test Room',
    description: 'A test room description',
    category: 'engineering',
    tags: ['backend', 'go'],
    message_count: 42,
    owner_id: 'user-uuid-123',
    created_at: '2026-01-01T00:00:00Z',
    last_active_at: '2026-04-01T00:00:00Z',
  };

  it('returns DiscussionForumPosting schema type', () => {
    const result = roomJsonLd({ room: baseRoom, url: 'https://solvr.dev/rooms/test-room' });
    expect(result['@type']).toBe('DiscussionForumPosting');
  });

  it('includes machineGeneratedContent in additionalProperty', () => {
    const result = roomJsonLd({ room: baseRoom, url: 'https://solvr.dev/rooms/test-room' });
    expect(result.additionalProperty).toBeDefined();
    const prop = result.additionalProperty as { name: string; value: boolean };
    expect(prop.name).toBe('machineGeneratedContent');
    expect(prop.value).toBe(true);
  });

  it('uses room.display_name as headline', () => {
    const result = roomJsonLd({ room: baseRoom, url: 'https://solvr.dev/rooms/test-room' });
    expect(result.headline).toBe('Test Room');
  });

  it('uses room.message_count in interactionStatistic', () => {
    const result = roomJsonLd({ room: baseRoom, url: 'https://solvr.dev/rooms/test-room' });
    const stat = result.interactionStatistic as { userInteractionCount: number };
    expect(stat.userInteractionCount).toBe(42);
  });

  it('handles missing description gracefully', () => {
    const roomWithoutDesc = { ...baseRoom, description: undefined };
    const result = roomJsonLd({ room: roomWithoutDesc, url: 'https://solvr.dev/rooms/test-room' });
    expect(result.description).toBe('A2A room on Solvr');
  });

  it('handles missing category (about field is undefined)', () => {
    const roomWithoutCategory = { ...baseRoom, category: undefined };
    const result = roomJsonLd({ room: roomWithoutCategory, url: 'https://solvr.dev/rooms/test-room' });
    expect(result.about).toBeUndefined();
  });
});
