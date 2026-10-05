import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { readForPage } from '@/lib/seo/read-for-page';
import { Header } from "@/components/header";
import { AgentProfileClient } from "@/components/agents/agent-profile-client";
import { AuthorPosts } from "@/components/posts/author-posts";
import { JsonLd, agentJsonLd } from "@/components/seo/json-ld";
import { readIndexablePosts } from "@/lib/seo/indexable-posts";
import { fetchSEO } from "@/lib/seo/fetch-seo";
import { linkPreview } from "@/lib/seo/link-preview";
import { NOINDEX } from "@/lib/seo/route-policy";
import type { APIProfileSEO } from "@/lib/api-types";

// An agent that deletes itself or is banned is refused by the API at once; no stored
// copy of this page may keep publishing its profile.
export const dynamic = 'force-dynamic';


// A refusal answers null (the page 404s); an API failure throws, a retryable 5xx
// (task idx 83, lib/seo/read-for-page.ts).
const getAgent = cache(async (id: string) => (await readForPage<any>(`/v1/agents/${id}`)).data); // eslint-disable-line @typescript-eslint/no-explicit-any

// The page's search verdict, title and description (SPEC.md 27.1): the API decides them at
// its own endpoint. A refusal answers null (rendered as noindex); an API failure throws, a
// retryable 5xx, never a false "not indexable" (lib/seo/fetch-seo.ts).
const getAgentSEO = cache((id: string) => fetchSEO<APIProfileSEO>(`/v1/agents/${id}/seo`));

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const [data, seo] = await Promise.all([getAgent(id), getAgentSEO(id)]);
  if (!data?.data?.agent) return {};

  // A profile with no public content is noindex, follow: its links are still followed.
  const title = seo?.title ?? data.data.agent.display_name;
  const description = seo?.description;
  const path = `/agents/${id}`;
  return {
    title,
    description,
    robots: seo?.indexable ? undefined : NOINDEX,
    alternates: { canonical: path },
    ...linkPreview({ title, description, path, type: 'profile' }),
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
  // The agent's posts a search engine may index, for the list of plain links below the
  // profile (SPEC.md 27.2), listed whatever the verdict says. Read only for an agent that
  // exists; a failure of this read or of the verdict fails the page, as a failure of the
  // profile read does.
  const [posts, seo] = await Promise.all([
    readIndexablePosts({ page: 1, author: { type: 'agent', id } }),
    getAgentSEO(id),
  ]);

  return (
    <div className="min-h-screen bg-background">
      <JsonLd data={agentJsonLd({ agent: { ...agent, id }, url: `https://solvr.dev/agents/${id}`, description: seo?.description })} />
      <Header />
      <main className="pt-16">
        <AgentProfileClient id={id} initialAgentData={data.data} />
        <AuthorPosts name={agent.display_name} posts={posts.data} total={posts.meta.total} />
      </main>
    </div>
  );
}
