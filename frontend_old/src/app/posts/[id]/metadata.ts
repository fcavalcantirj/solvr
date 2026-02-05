/**
 * Metadata generation for Post Detail pages
 * Per SPEC.md Part 19.2 SEO requirements
 * - Dynamic title: {post.title} | Solvr
 * - Description: First 160 chars of post description
 * - Open Graph tags for social sharing
 * - Twitter card tags
 */

import type { Metadata } from 'next';

// API base URL - defaults to localhost for development
const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

interface Post {
  id: string;
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  tags: string[];
  status: string;
  author: {
    type: 'human' | 'agent';
    id: string;
    display_name: string;
  };
  created_at: string;
}

interface MetadataProps {
  params: Promise<{ id: string }>;
}

/**
 * Fetch post data for metadata generation
 * Server-side fetch for SEO metadata
 */
async function getPost(id: string): Promise<Post | null> {
  try {
    const response = await fetch(`${API_URL}/v1/posts/${id}`, {
      next: { revalidate: 60 }, // Cache for 60 seconds
    });

    if (!response.ok) {
      return null;
    }

    const data = await response.json();
    return data.data || data;
  } catch {
    return null;
  }
}

/**
 * Truncate text to specified length, respecting word boundaries
 */
function truncateDescription(text: string, maxLength: number = 160): string {
  if (text.length <= maxLength) {
    return text;
  }

  // Find the last space before maxLength
  const truncated = text.substring(0, maxLength);
  const lastSpace = truncated.lastIndexOf(' ');

  if (lastSpace > maxLength * 0.7) {
    return truncated.substring(0, lastSpace) + '...';
  }

  return truncated + '...';
}

/**
 * Generate dynamic metadata for post detail pages
 * Per SPEC.md Part 19.2
 */
export async function generateMetadata({ params }: MetadataProps): Promise<Metadata> {
  const { id } = await params;
  const post = await getPost(id);

  if (!post) {
    return {
      title: 'Post Not Found | Solvr',
      description: 'The requested post could not be found.',
    };
  }

  const title = `${post.title} | Solvr`;
  const description = truncateDescription(post.description);

  return {
    title,
    description,
    keywords: post.tags,
    authors: [{ name: post.author.display_name }],
    openGraph: {
      title,
      description,
      type: 'article',
      siteName: 'Solvr',
      publishedTime: post.created_at,
      tags: post.tags,
    },
    twitter: {
      card: 'summary_large_image',
      title,
      description,
    },
  };
}
