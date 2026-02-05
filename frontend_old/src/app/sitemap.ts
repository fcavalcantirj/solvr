/**
 * Sitemap generation for Solvr
 * Per SPEC.md Part 19.2 - SEO
 *
 * Generates sitemap.xml with:
 * - Static pages (homepage, feed, search)
 * - Dynamic post URLs
 * - Proper priority and changefreq values
 */

import type { MetadataRoute } from 'next';

interface Post {
  id: string;
  type: string;
  title: string;
  updated_at: string;
}

interface PostsResponse {
  data: Post[];
  meta: {
    total: number;
    page?: number;
    per_page?: number;
    has_more?: boolean;
  };
}

/**
 * Get base URL from environment or default
 */
function getBaseUrl(): string {
  return process.env.NEXT_PUBLIC_APP_URL || 'https://solvr.dev';
}

/**
 * Get API URL from environment or default
 */
function getApiUrl(): string {
  return process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';
}

/**
 * Fetch all posts for sitemap inclusion
 */
async function fetchPosts(): Promise<Post[]> {
  try {
    const apiUrl = getApiUrl();
    // Fetch up to 1000 posts for sitemap (paginate if needed in production)
    const response = await fetch(`${apiUrl}/v1/posts?per_page=1000`, {
      next: { revalidate: 3600 }, // Cache for 1 hour
    });

    if (!response.ok) {
      console.error('Failed to fetch posts for sitemap:', response.status);
      return [];
    }

    const data: PostsResponse = await response.json();
    return data.data || [];
  } catch (error) {
    console.error('Error fetching posts for sitemap:', error);
    return [];
  }
}

/**
 * Generate sitemap with static pages and dynamic post URLs
 * Per SPEC.md Part 19.2:
 * - Homepage: priority 1.0, changefreq daily
 * - Feed: priority 0.9, changefreq hourly
 * - Posts: priority 0.7, changefreq weekly
 */
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const baseUrl = getBaseUrl();

  // Static pages per SPEC.md Part 19.2
  const staticPages: MetadataRoute.Sitemap = [
    {
      url: `${baseUrl}/`,
      lastModified: new Date().toISOString(),
      changeFrequency: 'daily',
      priority: 1.0,
    },
    {
      url: `${baseUrl}/feed`,
      lastModified: new Date().toISOString(),
      changeFrequency: 'hourly',
      priority: 0.9,
    },
    {
      url: `${baseUrl}/search`,
      lastModified: new Date().toISOString(),
      changeFrequency: 'daily',
      priority: 0.8,
    },
  ];

  // Fetch dynamic posts
  const posts = await fetchPosts();

  // Map posts to sitemap entries per SPEC.md Part 19.2
  const postPages: MetadataRoute.Sitemap = posts.map((post) => ({
    url: `${baseUrl}/posts/${post.id}`,
    lastModified: post.updated_at,
    changeFrequency: 'weekly' as const,
    priority: 0.7,
  }));

  return [...staticPages, ...postPages];
}
