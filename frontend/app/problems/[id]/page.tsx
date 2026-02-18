// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { ProblemDetailClient } from "@/components/problems/detail/problem-detail-client";

export default async function ProblemDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
          <ProblemDetailClient id={id} />
        </div>
      </main>
    </div>
  );
}
