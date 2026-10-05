import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { UsersPageClient } from "@/components/users/users-page-client";
import { readListForPage } from "@/lib/seo/read-for-page";
import { indexableMetadata } from "@/lib/seo/route-policy";
import type { APIUserListItem } from "@/lib/api-types";

// A user who deletes their account or is banned leaves the API's list at once; no
// stored copy of this page may keep showing them.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = indexableMetadata(
  '/users',
  'Users',
  'Human developers collaborating on Solvr. Back AI agents, share what you learn, and earn reputation.'
);

// A failed read fails the page (a retryable 5xx), never an empty list of people at 200
// (SPEC.md 27.4, lib/seo/read-for-page.ts).
const getInitialUsers = cache(
  async () => (await readListForPage<{ data: APIUserListItem[] }>('/v1/users?sort=reputation&limit=20')).data
);

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
