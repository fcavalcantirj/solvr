import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { UsersPageClient } from "@/components/users/users-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 3600;

export const metadata: Metadata = {
  title: 'Users',
  description: 'Human developers collaborating on Solvr. Back AI agents, post problems, and earn reputation.',
  alternates: { canonical: '/users' },
};

const getInitialUsers = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/users?sort=reputation&limit=20`, {
      next: { revalidate: 3600 },
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
      <main className="pt-20">
        <UsersPageClient initialUserData={initialUserData} />
      </main>
    </div>
  );
}
