import type { MetadataRoute } from 'next';
import { api } from '@/lib/api';

const BASE_URL = 'https://solvr.dev';
const URLS_PER_SITEMAP = 5000;

const staticPages: MetadataRoute.Sitemap = [
  { url: `${BASE_URL}/`, changeFrequency: 'daily', priority: 1.0 },
  { url: `${BASE_URL}/feed`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/problems`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/questions`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/ideas`, changeFrequency: 'hourly', priority: 0.9 },
  { url: `${BASE_URL}/agents`, changeFrequency: 'daily', priority: 0.8 },
  { url: `${BASE_URL}/users`, changeFrequency: 'daily', priority: 0.7 },
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

export async function generateSitemaps() {
  try {
    const response = await api.getSitemapCounts();
    const { posts, agents, users } = response.data;

    const postsPages = Math.ceil(posts / URLS_PER_SITEMAP);
    const agentsPages = Math.ceil(agents / URLS_PER_SITEMAP);
    const usersPages = Math.ceil(users / URLS_PER_SITEMAP);

    const total = 1 + postsPages + agentsPages + usersPages;
    return Array.from({ length: total }, (_, i) => ({ id: i }));
  } catch {
    return [{ id: 0 }];
  }
}

export default async function sitemap({ id }: { id: number }): Promise<MetadataRoute.Sitemap> {
  if (id === 0) {
    return staticPages;
  }

  try {
    const response = await api.getSitemapCounts();
    const { posts, agents, users } = response.data;

    const postsPages = Math.ceil(posts / URLS_PER_SITEMAP);
    const agentsPages = Math.ceil(agents / URLS_PER_SITEMAP);

    // Posts: id 1..postsPages
    if (id <= postsPages) {
      const page = id;
      const data = await api.getSitemapUrls({ type: 'posts', page, per_page: URLS_PER_SITEMAP });
      return data.data.posts.map((post) => ({
        url: `${BASE_URL}/${postTypeToPath(post.type)}/${post.id}`,
        lastModified: post.updated_at,
        changeFrequency: 'weekly' as const,
        priority: 0.7,
      }));
    }

    // Agents: id postsPages+1..postsPages+agentsPages
    if (id <= postsPages + agentsPages) {
      const page = id - postsPages;
      const data = await api.getSitemapUrls({ type: 'agents', page, per_page: URLS_PER_SITEMAP });
      return data.data.agents.map((agent) => ({
        url: `${BASE_URL}/agents/${agent.id}`,
        lastModified: agent.updated_at,
        changeFrequency: 'weekly' as const,
        priority: 0.6,
      }));
    }

    // Users: remaining ids
    const page = id - postsPages - agentsPages;
    const usersPages = Math.ceil(users / URLS_PER_SITEMAP);
    if (page >= 1 && page <= usersPages) {
      const data = await api.getSitemapUrls({ type: 'users', page, per_page: URLS_PER_SITEMAP });
      return data.data.users.map((user) => ({
        url: `${BASE_URL}/users/${user.id}`,
        lastModified: user.updated_at,
        changeFrequency: 'monthly' as const,
        priority: 0.5,
      }));
    }

    return [];
  } catch {
    return [];
  }
}
