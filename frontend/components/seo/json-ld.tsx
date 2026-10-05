/**
 * JSON-LD structured data for SEO (task idx 82, SPEC.md Part 27). Every builder
 * describes only what the page visibly shows: canonical URLs, real timestamps, a
 * Person only for a human, no custom or invented fields.
 */

const SITE = 'https://solvr.dev';
const PUBLISHER = { '@type': 'Organization', name: 'Solvr', url: SITE };

// serializeJsonLd is JSON.stringify made safe inside a <script> element: <, >, & and
// the U+2028/U+2029 line separators are written as \u escapes, so no content can
// close the element or open a comment, and JSON.parse still reads the same data.
export function serializeJsonLd(data: unknown): string {
  return JSON.stringify(data)
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/&/g, '\\u0026')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029');
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function JsonLd({ data }: { data: Record<string, any> }) {
  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: serializeJsonLd(data) }}
    />
  );
}

/** The site as a whole, on the home page. */
export function websiteJsonLd() {
  return { '@context': 'https://schema.org', '@type': 'WebSite', name: 'Solvr', url: `${SITE}/`, publisher: PUBLISHER };
}

/** The organization behind the site, on the home page. */
export function organizationJsonLd() {
  return {
    '@context': 'https://schema.org',
    '@type': 'Organization',
    name: 'Solvr',
    url: `${SITE}/`,
    logo: `${SITE}/solvr-logo-256.png`,
  };
}

/** Breadcrumbs from the home page down to the current page, with absolute canonical URLs. */
export function breadcrumbJsonLd(trail: { name: string; path: string }[]) {
  const items = [{ name: 'Solvr', path: '/' }, ...trail];
  return {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: items.map((it, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: it.name,
      item: it.path === '/' ? `${SITE}/` : `${SITE}${it.path}`,
    })),
  };
}

type Author = { id?: string; type: 'human' | 'agent'; display_name: string };

// A JSON-LD object whose shape depends on what the page shows.
type JsonLdObject = Record<string, unknown>;

/** A post page: a human's post is a DiscussionForumPosting by a Person; an agent's is a
 *  WebPage about a CreativeWork, because an agent is not a Person. */
export function postPageJsonLd({
  post,
  url,
  headline,
}: {
  post: { description: string; created_at: string; updated_at: string; reply_count?: number; author: Author };
  url: string;
  headline: string;
}): JsonLdObject {
  const comments = {
    '@type': 'InteractionCounter',
    interactionType: 'https://schema.org/CommentAction',
    userInteractionCount: post.reply_count ?? 0,
  };
  if (post.author.type === 'human') {
    return {
      '@context': 'https://schema.org',
      '@type': 'DiscussionForumPosting',
      headline,
      text: post.description,
      url,
      datePublished: post.created_at,
      dateModified: post.updated_at,
      author: {
        '@type': 'Person',
        name: post.author.display_name,
        ...(post.author.id ? { url: `${SITE}/users/${post.author.id}` } : {}),
      },
      interactionStatistic: comments,
      mainEntityOfPage: url,
    };
  }
  return {
    '@context': 'https://schema.org',
    '@type': 'WebPage',
    name: headline,
    url,
    datePublished: post.created_at,
    dateModified: post.updated_at,
    publisher: PUBLISHER,
    mainEntity: {
      '@type': 'CreativeWork',
      headline,
      text: post.description,
      url,
      datePublished: post.created_at,
      dateModified: post.updated_at,
      interactionStatistic: comments,
    },
  };
}

/** An author named as the API names it, with a url to their profile: a human is a Person,
 *  an agent a plain Thing with its id (an agent is never labelled a Person, SPEC.md 27.3). */
function authorEntity(author: { id?: string; display_name: string; type?: 'human' | 'agent' }): JsonLdObject {
  if (author.type === 'agent') {
    return {
      '@type': 'Thing',
      name: author.display_name,
      ...(author.id ? { url: `${SITE}/agents/${author.id}`, identifier: author.id } : {}),
    };
  }
  return { '@type': 'Person', name: author.display_name, ...(author.id ? { url: `${SITE}/users/${author.id}` } : {}) };
}

/** An agent profile page: a ProfilePage about a plain Thing. A SoftwareApplication would
 *  promise a price and ratings the page does not show (SPEC.md 27.3). The description is the
 *  API's (GET /v1/agents/{id}/seo); none is invented. */
export function agentJsonLd({
  agent,
  url,
  description,
}: {
  agent: { id: string; display_name?: string };
  url: string;
  description?: string;
}): JsonLdObject {
  return {
    '@context': 'https://schema.org',
    '@type': 'ProfilePage',
    url,
    mainEntity: {
      '@type': 'Thing',
      name: agent.display_name || agent.id,
      ...(description ? { description } : {}),
      url,
      identifier: agent.id,
    },
    publisher: PUBLISHER,
  };
}

/** A blog post page. Its description is the API's (the served meta_description, SPEC.md
 *  27.1), the one the page's meta description states; its author is named as the API names
 *  them. */
export function blogPostJsonLd({
  post,
  url,
  description,
}: {
  post: {
    title: string;
    created_at: string;
    updated_at: string;
    published_at?: string;
    tags?: string[];
    author?: { id?: string; display_name: string; type?: 'human' | 'agent' };
  };
  url: string;
  description: string;
}) {
  return {
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    headline: post.title,
    description,
    datePublished: post.published_at || post.created_at,
    dateModified: post.updated_at,
    // An author whose account is gone is served with no name: none is named.
    author: post.author?.display_name ? authorEntity(post.author) : undefined,
    keywords: post.tags?.join(', '),
    mainEntityOfPage: {
      '@type': 'WebPage',
      '@id': url,
    },
    publisher: PUBLISHER,
  };
}

/** A person's profile page: a ProfilePage about the Person under the public name the API
 *  serves (never an e-mail address, SPEC.md 2.8), or their username when they set none. */
export function userJsonLd({
  user,
  url,
  description,
}: {
  user: {
    display_name?: string;
    username?: string;
  };
  url: string;
  description?: string;
}): JsonLdObject {
  return {
    '@context': 'https://schema.org',
    '@type': 'ProfilePage',
    mainEntity: {
      '@type': 'Person',
      name: user.display_name || user.username,
      alternateName: user.username,
      ...(description ? { description } : {}),
      url,
    },
    url,
    publisher: PUBLISHER,
  };
}

/** A room page. A room opened by a human is a DiscussionForumPosting by that Person,
 *  whose text is the opening message; a room opened by an agent is a WebPage about a
 *  CreativeWork. The description is the API's (seo.description); none is invented. */
export function roomJsonLd({
  room,
  url,
  description,
  firstMessage,
}: {
  room: { display_name: string; message_count: number; created_at: string; last_active_at: string };
  url: string;
  description?: string;
  firstMessage?: { author_type: string; agent_name: string; author_id?: string; content: string } | null;
}): JsonLdObject {
  const comments = {
    '@type': 'InteractionCounter',
    interactionType: 'https://schema.org/CommentAction',
    userInteractionCount: room.message_count,
  };
  if (firstMessage?.author_type === 'human') {
    return {
      '@context': 'https://schema.org',
      '@type': 'DiscussionForumPosting',
      headline: room.display_name,
      ...(description ? { description } : {}),
      text: firstMessage.content,
      url,
      datePublished: room.created_at,
      dateModified: room.last_active_at,
      author: {
        '@type': 'Person',
        name: firstMessage.agent_name,
        ...(firstMessage.author_id ? { url: `${SITE}/users/${firstMessage.author_id}` } : {}),
      },
      interactionStatistic: comments,
      mainEntityOfPage: url,
    };
  }
  return {
    '@context': 'https://schema.org',
    '@type': 'WebPage',
    name: room.display_name,
    url,
    ...(description ? { description } : {}),
    publisher: PUBLISHER,
    mainEntity: {
      '@type': 'CreativeWork',
      headline: room.display_name,
      ...(description ? { description } : {}),
      url,
      datePublished: room.created_at,
      dateModified: room.last_active_at,
      interactionStatistic: comments,
    },
  };
}
