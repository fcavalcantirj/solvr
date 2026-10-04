import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { readForPage } from '@/lib/seo/read-for-page';
import { Header } from "@/components/header";
import { UserProfileClient } from "@/components/users/user-profile-client";
import { JsonLd, userJsonLd } from "@/components/seo/json-ld";

// A user who deletes their account or is banned is refused by the API at once; no
// stored copy of this page may keep publishing their profile.
export const dynamic = 'force-dynamic';


// A refusal answers null (the page 404s); an API failure throws, a retryable 5xx
// (task idx 83, lib/seo/read-for-page.ts).
const getUser = cache(async (id: string) => (await readForPage<any>(`/v1/users/${id}`)).data); // eslint-disable-line @typescript-eslint/no-explicit-any

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const data = await getUser(id);
  if (!data?.data) return {};

  const user = data.data;
  const displayName = user.display_name || user.username || 'User';
  const description = user.bio
    ? user.bio.replace(/[#*`\[\]]/g, '').slice(0, 160)
    : `${displayName} on Solvr`;

  return {
    title: displayName,
    description,
    openGraph: {
      title: displayName,
      description,
      type: 'profile',
    },
    twitter: {
      card: 'summary',
      title: displayName,
      description,
    },
    alternates: {
      canonical: `/users/${id}`,
    },
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

  return (
    <div className="min-h-screen bg-background">
      <JsonLd data={userJsonLd({ user, url: `https://solvr.dev/users/${id}` })} />
      <Header />
      <main className="pt-16">
        <UserProfileClient id={id} initialUserData={user} />
      </main>
    </div>
  );
}
