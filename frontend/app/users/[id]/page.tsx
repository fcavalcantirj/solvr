import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { readForPage } from '@/lib/seo/read-for-page';
import { Header } from "@/components/header";
import { UserProfileClient } from "@/components/users/user-profile-client";
import { AuthorPosts } from "@/components/posts/author-posts";
import { JsonLd, userJsonLd } from "@/components/seo/json-ld";
import { readIndexablePosts } from "@/lib/seo/indexable-posts";
import { fetchSEO } from "@/lib/seo/fetch-seo";
import { linkPreview } from "@/lib/seo/link-preview";
import { NOINDEX } from "@/lib/seo/route-policy";
import type { APIProfileSEO } from "@/lib/api-types";

// A user who deletes their account or is banned is refused by the API at once; no
// stored copy of this page may keep publishing their profile.
export const dynamic = 'force-dynamic';


// A refusal answers null (the page 404s); an API failure throws, a retryable 5xx
// (task idx 83, lib/seo/read-for-page.ts).
const getUser = cache(async (id: string) => (await readForPage<any>(`/v1/users/${id}`)).data); // eslint-disable-line @typescript-eslint/no-explicit-any

// The page's search verdict, title and description (SPEC.md 27.1): the API decides them at
// its own endpoint. A refusal answers null (rendered as noindex); an API failure throws, a
// retryable 5xx, never a false "not indexable" (lib/seo/fetch-seo.ts).
const getUserSEO = cache((id: string) => fetchSEO<APIProfileSEO>(`/v1/users/${id}/seo`));

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const [data, seo] = await Promise.all([getUser(id), getUserSEO(id)]);
  if (!data?.data) return {};

  // A profile with no public content is noindex, follow: its links are still followed.
  // The API names the person by their public name, never an e-mail address (SPEC.md 2.8).
  const user = data.data;
  const title = seo?.title ?? (user.display_name || user.username || 'User');
  const description = seo?.description;
  const path = `/users/${id}`;
  return {
    title,
    description,
    robots: seo?.indexable ? undefined : NOINDEX,
    alternates: { canonical: path },
    ...linkPreview({ title, description, path, type: 'profile' }),
  };
}

export default async function UserProfilePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const data = await getUser(id);

  if (!data?.data) notFound();

  const user = data.data;
  // The person's posts a search engine may index, for the list of plain links below the
  // profile (SPEC.md 27.2), listed whatever the verdict says. Read only for a user that
  // exists; a failure of this read or of the verdict fails the page, as a failure of the
  // profile read does.
  const [posts, seo] = await Promise.all([
    readIndexablePosts({ page: 1, author: { type: 'human', id } }),
    getUserSEO(id),
  ]);

  return (
    <div className="min-h-screen bg-background">
      <JsonLd data={userJsonLd({ user, url: `https://solvr.dev/users/${id}`, description: seo?.description })} />
      <Header />
      <main className="pt-16">
        <UserProfileClient id={id} initialUserData={user} />
        <AuthorPosts name={user.display_name || user.username || 'this user'} posts={posts.data} total={posts.meta.total} />
      </main>
    </div>
  );
}
