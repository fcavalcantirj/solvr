"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Header } from "@/components/header";
import { IdeasFilters, IdeasFilterStats } from "@/components/ideas/ideas-filters";
import { IdeasList } from "@/components/ideas/ideas-list";
import { IdeasSidebar } from "@/components/ideas/ideas-sidebar";
import { Lightbulb, Plus, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useIdeasStats } from "@/hooks/use-ideas-stats";
import { useAuth } from "@/hooks/use-auth";

function formatNumber(num: number): string {
  if (num >= 1000) {
    return (num / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  }
  return num.toLocaleString();
}

// Map frontend stage names to API status values
function mapStageToStatus(stage: string): string | undefined {
  const stageMap: Record<string, string> = {
    spark: 'open',
    developing: 'active',
    mature: 'dormant',
    realized: 'evolved',
  };
  if (stage === 'all') return undefined;
  return stageMap[stage] || stage;
}

export default function IdeasPage() {
  const router = useRouter();
  const { isAuthenticated } = useAuth();
  const { stats, loading } = useIdeasStats();

  // Lifted filter state (same pattern as problems/page.tsx and questions/page.tsx)
  const [stage, setStage] = useState<string>('all');
  const [sort, setSort] = useState<string>('newest');
  const [tags, setTags] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState("");

  const handleSparkIdea = () => {
    if (isAuthenticated) {
      router.push('/ideas/new');
    } else {
      router.push('/login?next=/ideas/new');
    }
  };

  // Derive filter stats from the stats hook
  const filterStats: IdeasFilterStats | undefined = stats ? {
    total: stats.total,
    spark: stats.countsByStatus.spark ?? 0,
    developing: stats.countsByStatus.developing ?? 0,
    mature: stats.countsByStatus.mature ?? 0,
    realized: stats.countsByStatus.realized ?? 0,
    archived: stats.countsByStatus.archived ?? 0,
  } : undefined;

  // Map stage to API status for IdeasList
  const apiStatus = mapStageToStatus(stage);

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border overflow-hidden">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
              <div>
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-10 h-10 bg-foreground flex items-center justify-center shrink-0">
                    <Lightbulb className="w-5 h-5 text-background" />
                  </div>
                  <span className="font-mono text-xs tracking-wider text-muted-foreground">
                    COLLECTIVE INNOVATION
                  </span>
                </div>
                <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
                  IDEAS
                </h1>
                <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-xl">
                  Seeds of possibility. Sparks before the fire. The raw, unpolished thoughts that could become breakthroughs.
                </p>
              </div>
              <Button
                className="font-mono text-xs tracking-wider w-full sm:w-auto shrink-0 hidden md:flex"
                onClick={handleSparkIdea}
              >
                <Plus className="w-3 h-3 mr-2" />
                SPARK IDEA
              </Button>
            </div>

            {/* Quick Stats */}
            <div className="grid grid-cols-2 sm:flex sm:items-center gap-4 sm:gap-8 mt-8 pt-6 border-t border-border">
              {loading ? (
                <div className="flex items-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
                  <span className="font-mono text-xs text-muted-foreground">Loading stats...</span>
                </div>
              ) : (
                <>
                  <div className="flex flex-col sm:flex-row sm:items-baseline">
                    <span className="font-mono text-xl sm:text-2xl font-medium text-foreground">
                      {formatNumber(stats?.total ?? 0)}
                    </span>
                    <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">TOTAL</span>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-baseline">
                    <span className="font-mono text-xl sm:text-2xl font-medium text-amber-600">
                      {formatNumber(stats?.countsByStatus.spark ?? 0)}
                    </span>
                    <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">SPARKS</span>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-baseline">
                    <span className="font-mono text-xl sm:text-2xl font-medium text-blue-600">
                      {formatNumber(stats?.countsByStatus.developing ?? 0)}
                    </span>
                    <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">DEVELOPING</span>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-baseline">
                    <span className="font-mono text-xl sm:text-2xl font-medium text-emerald-600">
                      {formatNumber(stats?.countsByStatus.realized ?? 0)}
                    </span>
                    <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">REALIZED</span>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>

        {/* Filters */}
        <IdeasFilters
          stats={filterStats}
          stage={stage}
          sort={sort}
          tags={tags}
          searchQuery={searchQuery}
          onStageChange={setStage}
          onSortChange={setSort}
          onTagsChange={setTags}
          onSearchQueryChange={setSearchQuery}
        />

        {/* Main Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            <div className="lg:col-span-2">
              <IdeasList options={{ status: apiStatus, sort: sort as 'newest' | 'trending' | 'most_support', tags, searchQuery }} />
            </div>
            <div className="lg:col-span-1">
              <IdeasSidebar />
            </div>
          </div>
        </div>
      </main>

      {/* Mobile CTA */}
      <div className="md:hidden fixed bottom-6 left-6 right-6">
        <Button
          className="w-full font-mono text-xs tracking-wider"
          onClick={handleSparkIdea}
        >
          <Plus className="w-3 h-3 mr-2" />
          SPARK IDEA
        </Button>
      </div>
    </div>
  );
}
