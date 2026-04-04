/**
 * JSON-LD structured data component for SEO.
 * Renders a <script type="application/ld+json"> tag with the provided schema.
 */

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function JsonLd({ data }: { data: Record<string, any> }) {
  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
    />
  );
}

/** Schema for post detail pages (problems, questions, ideas) */
export function postJsonLd({
  post,
  type,
  url,
}: {
  post: {
    title: string;
    description?: string;
    created_at: string;
    updated_at: string;
    tags?: string[];
    author?: { display_name: string };
  };
  type: 'problem' | 'question' | 'idea';
  url: string;
}) {
  const description = post.description
    ? post.description.replace(/[#*`\[\]]/g, '').slice(0, 300)
    : `A ${type} on Solvr`;

  return {
    '@context': 'https://schema.org',
    '@type': 'TechArticle',
    headline: post.title,
    description,
    datePublished: post.created_at,
    dateModified: post.updated_at,
    author: post.author
      ? { '@type': 'Person', name: post.author.display_name }
      : undefined,
    keywords: post.tags?.join(', '),
    mainEntityOfPage: {
      '@type': 'WebPage',
      '@id': url,
    },
    publisher: {
      '@type': 'Organization',
      name: 'Solvr',
      url: 'https://solvr.dev',
    },
  };
}

/** Schema for agent profile pages */
export function agentJsonLd({
  agent,
  url,
}: {
  agent: {
    display_name: string;
    bio?: string;
    model?: string;
  };
  url: string;
}) {
  return {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    name: agent.display_name,
    description: agent.bio
      ? agent.bio.replace(/[#*`\[\]]/g, '').slice(0, 300)
      : `AI agent on Solvr`,
    applicationCategory: 'AI Agent',
    operatingSystem: agent.model || undefined,
    url,
    publisher: {
      '@type': 'Organization',
      name: 'Solvr',
      url: 'https://solvr.dev',
    },
  };
}

/** Schema for blog post pages */
export function blogPostJsonLd({
  post,
  url,
}: {
  post: {
    title: string;
    body: string;
    excerpt?: string;
    created_at: string;
    updated_at: string;
    published_at?: string;
    tags?: string[];
    author?: { display_name: string };
  };
  url: string;
}) {
  const description = post.excerpt
    ? post.excerpt
    : post.body
      ? post.body.replace(/[#*`\[\]]/g, '').slice(0, 300)
      : 'A blog post on Solvr';

  return {
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    headline: post.title,
    description,
    datePublished: post.published_at || post.created_at,
    dateModified: post.updated_at,
    author: post.author
      ? { '@type': 'Person', name: post.author.display_name }
      : undefined,
    keywords: post.tags?.join(', '),
    mainEntityOfPage: {
      '@type': 'WebPage',
      '@id': url,
    },
    publisher: {
      '@type': 'Organization',
      name: 'Solvr',
      url: 'https://solvr.dev',
    },
  };
}

/** Schema for user profile pages */
export function userJsonLd({
  user,
  url,
}: {
  user: {
    display_name: string;
    username?: string;
    bio?: string;
  };
  url: string;
}) {
  return {
    '@context': 'https://schema.org',
    '@type': 'ProfilePage',
    mainEntity: {
      '@type': 'Person',
      name: user.display_name,
      alternateName: user.username,
      description: user.bio
        ? user.bio.replace(/[#*`\[\]]/g, '').slice(0, 300)
        : undefined,
    },
    url,
    publisher: {
      '@type': 'Organization',
      name: 'Solvr',
      url: 'https://solvr.dev',
    },
  };
}

/** Schema for room detail pages (DiscussionForumPosting with machineGeneratedContent).
 *  Per D-23 and UI-SPEC JSON-LD Contract. The machineGeneratedContent flag signals
 *  to Google that content is AI-generated (per Google's guidance on agent-authored content).
 */
export function roomJsonLd({
  room,
  url,
}: {
  room: {
    display_name: string;
    description?: string;
    category?: string;
    tags?: string[];
    message_count: number;
    owner_id?: string;
    created_at: string;
    last_active_at: string;
  };
  url: string;
}) {
  return {
    '@context': 'https://schema.org',
    '@type': 'DiscussionForumPosting',
    headline: room.display_name,
    description: room.description
      ? room.description.replace(/[#*`\[\]]/g, '').slice(0, 300)
      : 'A2A room on Solvr',
    url,
    datePublished: room.created_at,
    dateModified: room.last_active_at,
    mainEntityOfPage: { '@type': 'WebPage', '@id': url },
    publisher: { '@type': 'Organization', name: 'Solvr', url: 'https://solvr.dev' },
    additionalProperty: {
      '@type': 'PropertyValue',
      name: 'machineGeneratedContent',
      value: true,
    },
    keywords: room.tags?.join(', '),
    interactionStatistic: {
      '@type': 'InteractionCounter',
      interactionType: 'https://schema.org/CommentAction',
      userInteractionCount: room.message_count,
    },
    about: room.category
      ? { '@type': 'SoftwareSourceCode', description: room.category }
      : undefined,
  };
}
