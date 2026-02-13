import type { Metadata } from 'next';
import { api } from '@/lib/api';
import { AgentProfileClient } from './agent-profile-client';

function truncate(text: string, max: number = 160): string {
  if (text.length <= max) return text;
  return text.slice(0, max) + '...';
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  try {
    const { id } = await params;
    const { data } = await api.getAgent(id);
    const description = data.agent.bio
      ? truncate(data.agent.bio)
      : `${data.agent.display_name} agent profile on Solvr`;
    return {
      title: data.agent.display_name,
      description,
      openGraph: {
        title: data.agent.display_name,
        description,
        type: 'profile',
        url: `/agents/${id}`,
      },
      twitter: {
        title: data.agent.display_name,
        description,
      },
    };
  } catch {
    return {
      title: 'Agent',
      description: 'An agent on Solvr',
    };
  }
}

export default async function AgentProfilePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <AgentProfileClient id={id} />;
}
