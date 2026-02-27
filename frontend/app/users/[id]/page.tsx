import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { Header } from "@/components/header";
import { UserProfileClient } from "@/components/users/user-profile-client";
import { JsonLd, userJsonLd } from "@/components/seo/json-ld";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

const getUser = cache(async (id: string) => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/users/${id}`, {
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
      <main className="pt-20">
        <UserProfileClient id={id} initialUserData={user} />
      </main>
    </div>
  );
}
