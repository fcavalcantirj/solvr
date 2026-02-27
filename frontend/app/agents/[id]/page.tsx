import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { Header } from "@/components/header";
import { AgentProfileClient } from "@/components/agents/agent-profile-client";
import { JsonLd, agentJsonLd } from "@/components/seo/json-ld";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

const getAgent = cache(async (id: string) => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/agents/${id}`, {
      next: { revalidate: 3600 },
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
});

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const data = await getAgent(id);
  if (!data?.data?.agent) return {};

  const agent = data.data.agent;
  const description = agent.bio
    ? agent.bio.replace(/[#*`\[\]]/g, '').slice(0, 160)
    : `AI agent on Solvr`;

  return {
    title: agent.display_name,
    description,
    openGraph: {
      title: agent.display_name,
      description,
      type: 'profile',
    },
    twitter: {
      card: 'summary',
      title: agent.display_name,
      description,
    },
    alternates: {
      canonical: `/agents/${id}`,
    },
  };
}

export default async function AgentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const data = await getAgent(id);

  if (!data?.data?.agent) notFound();

  const agent = data.data.agent;

  return (
    <div className="min-h-screen bg-background">
      <JsonLd data={agentJsonLd({ agent, url: `https://solvr.dev/agents/${id}` })} />
      <Header />
      <main className="pt-20">
        <AgentProfileClient id={id} initialAgentData={data.data} />
      </main>
    </div>
  );
}
