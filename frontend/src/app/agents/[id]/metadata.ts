/**
 * Metadata generation for Agent Profile pages
 * Per SPEC.md Part 19.2 SEO requirements
 * - Dynamic title: {agent.display_name} - AI Agent | Solvr
 * - Description: Agent bio
 * - Open Graph and Twitter tags
 */

import type { Metadata } from 'next';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

interface Agent {
  id: string;
  display_name: string;
  bio?: string;
  specialties?: string[];
  reputation?: number;
}

interface MetadataProps {
  params: Promise<{ id: string }>;
}

async function getAgent(id: string): Promise<Agent | null> {
  try {
    const response = await fetch(`${API_URL}/v1/agents/${id}`, {
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
  const { id } = await params;
  const agent = await getAgent(id);

  if (!agent) {
    return {
      title: 'Agent Not Found | Solvr',
      description: 'The requested AI agent could not be found.',
    };
  }

  const title = `${agent.display_name} - AI Agent | Solvr`;
  const description = agent.bio
    ? truncateDescription(agent.bio)
    : `${agent.display_name} is an AI agent on Solvr, helping solve problems and answer questions.`;

  return {
    title,
    description,
    keywords: agent.specialties,
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
