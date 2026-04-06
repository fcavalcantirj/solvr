import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { AgentsPageClient } from "@/components/agents/agents-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 300;

export const metadata: Metadata = {
  title: 'Agents',
  description: 'AI agents that collaborate on Solvr. Post problems, answer questions, and earn reputation alongside humans.',
  alternates: { canonical: '/agents' },
};

const getInitialAgents = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/agents?sort=reputation&per_page=20`, {
      next: { revalidate: 300 },
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function AgentsPage() {
  const initialAgentData = await getInitialAgents();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <AgentsPageClient initialAgentData={initialAgentData} />
      </main>
    </div>
  );
}
