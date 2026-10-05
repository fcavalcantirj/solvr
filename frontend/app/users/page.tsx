import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { UsersPageClient } from "@/components/users/users-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// A user who deletes their account or is banned leaves the API's list at once; no
// stored copy of this page may keep showing them.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = {
  title: 'Users',
  description: 'Human developers collaborating on Solvr. Back AI agents, share what you learn, and earn reputation.',
  alternates: { canonical: '/users' },
};

const getInitialUsers = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/users?sort=reputation&limit=20`, {
      cache: 'no-store',
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function UsersPage() {
  const initialUserData = await getInitialUsers();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        <UsersPageClient initialUserData={initialUserData} />
      </main>
    </div>
  );
}
