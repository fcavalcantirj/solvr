/**
 * Metadata generation for User Profile pages
 * Per SPEC.md Part 19.2 SEO requirements
 * - Dynamic title: {user.display_name} | Solvr
 * - Description: User bio
 * - Open Graph and Twitter tags
 */

import type { Metadata } from 'next';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

interface User {
  id: string;
  username: string;
  display_name: string;
  bio?: string;
  reputation?: number;
}

interface MetadataProps {
  params: Promise<{ username: string }>;
}

async function getUser(username: string): Promise<User | null> {
  try {
    const response = await fetch(`${API_URL}/v1/users/${username}`, {
      next: { revalidate: 60 },
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

function truncateDescription(text: string, maxLength: number = 160): string {
  if (text.length <= maxLength) {
    return text;
  }
  const truncated = text.substring(0, maxLength);
  const lastSpace = truncated.lastIndexOf(' ');
  if (lastSpace > maxLength * 0.7) {
    return truncated.substring(0, lastSpace) + '...';
  }
  return truncated + '...';
}

export async function generateMetadata({ params }: MetadataProps): Promise<Metadata> {
  const { username } = await params;
  const user = await getUser(username);

  if (!user) {
    return {
      title: 'User Not Found | Solvr',
      description: 'The requested user could not be found.',
    };
  }

  const title = `${user.display_name} | Solvr`;
  const description = user.bio
    ? truncateDescription(user.bio)
    : `${user.display_name} is a member of Solvr, the knowledge base for developers and AI agents.`;

  return {
    title,
    description,
    openGraph: {
      title,
      description,
      type: 'profile',
      siteName: 'Solvr',
    },
    twitter: {
      card: 'summary_large_image',
      title,
      description,
    },
  };
}
