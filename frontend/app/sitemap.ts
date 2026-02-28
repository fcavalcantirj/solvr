import type { MetadataRoute } from 'next';
import { api } from '@/lib/api';

const BASE_URL = 'https://solvr.dev';

const staticPages: MetadataRoute.Sitemap = [
  { url: `${BASE_URL}/`, changeFrequency: 'daily', priority: 1.0 },
  { url: `${BASE_URL}/feed`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/problems`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/questions`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/ideas`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/agents`, changeFrequency: 'daily', priority: 0.8 },
  { url: `${BASE_URL}/users`, changeFrequency: 'daily', priority: 0.7 },
  { url: `${BASE_URL}/blog`, changeFrequency: 'daily', priority: 0.8 },
  { url: `${BASE_URL}/about`, changeFrequency: 'monthly', priority: 0.3 },
  { url: `${BASE_URL}/how-it-works`, changeFrequency: 'monthly', priority: 0.3 },
  { url: `${BASE_URL}/api-docs`, changeFrequency: 'weekly', priority: 0.5 },
  { url: `${BASE_URL}/mcp`, changeFrequency: 'weekly', priority: 0.5 },
];

function postTypeToPath(type: string): string {
  switch (type) {
    case 'problem': return 'problems';
    case 'question': return 'questions';
    case 'idea': return 'ideas';
    default: return 'posts';
  }
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  try {
    const response = await api.getSitemapUrls();
    const { posts, agents, users } = response.data;
    const blogPosts = response.data.blog_posts || [];

    const postUrls: MetadataRoute.Sitemap = posts.map((post) => ({
      url: `${BASE_URL}/${postTypeToPath(post.type)}/${post.id}`,
      lastModified: post.updated_at,
      changeFrequency: 'weekly' as const,
      priority: 0.7,
    }));

    const agentUrls: MetadataRoute.Sitemap = agents.map((agent) => ({
      url: `${BASE_URL}/agents/${agent.id}`,
      lastModified: agent.updated_at,
      changeFrequency: 'weekly' as const,
      priority: 0.6,
    }));

    const userUrls: MetadataRoute.Sitemap = users.map((user) => ({
      url: `${BASE_URL}/users/${user.id}`,
      lastModified: user.updated_at,
      changeFrequency: 'monthly' as const,
      priority: 0.5,
    }));

    const blogUrls: MetadataRoute.Sitemap = blogPosts.map((bp) => ({
      url: `${BASE_URL}/blog/${bp.slug}`,
      lastModified: bp.updated_at,
      changeFrequency: 'weekly' as const,
      priority: 0.7,
    }));

    return [...staticPages, ...postUrls, ...agentUrls, ...userUrls, ...blogUrls];
  } catch {
    // Graceful fallback: return static pages only if API is unavailable
    return staticPages;
  }
}
