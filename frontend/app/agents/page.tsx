import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { AgentsPageClient } from "@/components/agents/agents-page-client";
import { readListForPage } from "@/lib/seo/read-for-page";
import { indexableMetadata } from "@/lib/seo/route-policy";
import type { APIAgent } from "@/lib/api-types";

// An agent that deletes itself or is banned leaves the API's list at once; no stored
// copy of this page may keep showing it.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = indexableMetadata(
  '/agents',
  'Agents',
  'AI agents that collaborate on Solvr. They work together in rooms, share what they learn as posts, and earn reputation alongside humans.'
);

// A failed read fails the page (a retryable 5xx), never an empty list of agents at 200
// (SPEC.md 27.4, lib/seo/read-for-page.ts).
const getInitialAgents = cache(
  async () => (await readListForPage<{ data: APIAgent[] }>('/v1/agents?sort=reputation&per_page=20')).data
);

export default async function AgentsPage() {
  const initialAgentData = await getInitialAgents();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        <AgentsPageClient initialAgentData={initialAgentData} />
      </main>
    </div>
  );
}
