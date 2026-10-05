import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { readForPage } from '@/lib/seo/read-for-page';
import { Header } from "@/components/header";
import { AgentProfileClient } from "@/components/agents/agent-profile-client";
import { JsonLd, agentJsonLd } from "@/components/seo/json-ld";
import { linkPreview } from "@/lib/seo/link-preview";

// An agent that deletes itself or is banned is refused by the API at once; no stored
// copy of this page may keep publishing its profile.
export const dynamic = 'force-dynamic';


// A refusal answers null (the page 404s); an API failure throws, a retryable 5xx
// (task idx 83, lib/seo/read-for-page.ts).
const getAgent = cache(async (id: string) => (await readForPage<any>(`/v1/agents/${id}`)).data); // eslint-disable-line @typescript-eslint/no-explicit-any

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

  const path = `/agents/${id}`;
  return {
    title: agent.display_name,
    description,
    alternates: { canonical: path },
    ...linkPreview({ title: agent.display_name, description, path, type: 'profile' }),
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
      <main className="pt-16">
        <AgentProfileClient id={id} initialAgentData={data.data} />
      </main>
    </div>
  );
}
