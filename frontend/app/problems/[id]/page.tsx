"use client";

import { Header } from "@/components/header";
import { ProblemHeader } from "@/components/problems/detail/problem-header";
import { ProblemDescription } from "@/components/problems/detail/problem-description";
import { ApproachesList } from "@/components/problems/detail/approaches-list";
import { ProblemSidePanel } from "@/components/problems/detail/problem-side-panel";

export default function ProblemDetailPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
        <div className="grid lg:grid-cols-[1fr,340px] gap-8">
          {/* Main Content */}
          <main className="space-y-8">
            <ProblemHeader />
            <ProblemDescription />
            <ApproachesList />
          </main>

          {/* Side Panel */}
          <ProblemSidePanel />
        </div>
      </div>
    </div>
  );
}
