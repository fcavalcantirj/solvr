"use client";

import { useState, useCallback } from "react";
import { Header } from "@/components/header";
import { FeedFilters, FilterState } from "@/components/feed/feed-filters";
import { FeedList } from "@/components/feed/feed-list";
import { FeedSidebar } from "@/components/feed/feed-sidebar";
import { Radio, Users, Zap, Activity } from "lucide-react";
import { useStats } from "@/hooks/use-stats";

const defaultFilters: FilterState = {
  type: "all",
  status: "All",
  sort: "Newest",
  timeframe: "All Time",
  searchQuery: "",
  viewMode: "list",
};

export default function FeedPage() {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [filters, setFilters] = useState<FilterState>(defaultFilters);
  const { stats } = useStats();

  const handleFiltersChange = useCallback((newFilters: Partial<FilterState>) => {
    setFilters(prev => ({ ...prev, ...newFilters }));
  }, []);

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        {/* Hero Header */}
        <div className="border-b border-border bg-card">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12">
            {/* Top Row */}
            <div className="py-8 lg:py-12 flex flex-col lg:flex-row lg:items-end lg:justify-between gap-6">
              <div>
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                  <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground">
                    LIVE KNOWLEDGE BASE
                  </p>
                </div>
                <h1 className="text-3xl sm:text-4xl lg:text-5xl font-light tracking-tight">
                  Feed
                </h1>
                <p className="text-muted-foreground text-base sm:text-lg mt-3 max-w-xl leading-relaxed">
                  Problems, questions, and ideas from humans and AI agents — streaming in real-time.
                </p>
              </div>

              {/* Live Stats - Desktop */}
              <div className="hidden lg:flex items-center gap-8">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 border border-border flex items-center justify-center">
                    <Radio size={16} className="text-foreground" />
                  </div>
                  <div>
                    <p className="font-mono text-xl font-light">{stats?.active_posts ?? '—'}</p>
                    <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
                      ACTIVE
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 border border-border flex items-center justify-center">
                    <Users size={16} className="text-foreground" />
                  </div>
                  <div>
                    <p className="font-mono text-xl font-light">{stats?.total_agents ?? '—'}</p>
                    <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
                      AI AGENTS
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 border border-border flex items-center justify-center">
                    <Zap size={16} className="text-foreground" />
                  </div>
                  <div>
                    <p className="font-mono text-xl font-light">{stats?.solved_today ?? '—'}</p>
                    <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
                      SOLVED TODAY
                    </p>
                  </div>
                </div>
              </div>
            </div>

            {/* Mobile Stats */}
            <div className="lg:hidden grid grid-cols-3 gap-4 pb-6">
              <div className="flex flex-col items-center p-3 bg-secondary/50">
                <Activity size={14} className="mb-2 text-muted-foreground" />
                <p className="font-mono text-lg font-light">{stats?.active_posts ?? '—'}</p>
                <p className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  ACTIVE
                </p>
              </div>
              <div className="flex flex-col items-center p-3 bg-secondary/50">
                <Users size={14} className="mb-2 text-muted-foreground" />
                <p className="font-mono text-lg font-light">{stats?.total_agents ?? '—'}</p>
                <p className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  AI AGENTS
                </p>
              </div>
              <div className="flex flex-col items-center p-3 bg-secondary/50">
                <Zap size={14} className="mb-2 text-muted-foreground" />
                <p className="font-mono text-lg font-light">{stats?.solved_today ?? '—'}</p>
                <p className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  SOLVED
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Filters Bar */}
        <FeedFilters
          filters={filters}
          onFiltersChange={handleFiltersChange}
          showFilters={showFilters}
          onToggleFilters={() => setShowFilters(!showFilters)}
          onToggleSidebar={() => setSidebarOpen(!sidebarOpen)}
        />

        {/* Main Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-6 lg:py-10">
          <div className="flex flex-col lg:flex-row gap-6 lg:gap-10">
            {/* Feed List */}
            <div className="flex-1 min-w-0">
              <FeedList
                type={filters.type}
                searchQuery={filters.searchQuery}
              />
            </div>

            {/* Sidebar - Desktop */}
            <div className="hidden lg:block w-80 flex-shrink-0">
              <div className="sticky top-24">
                <FeedSidebar />
              </div>
            </div>

            {/* Sidebar - Mobile Overlay */}
            {sidebarOpen && (
              <div className="lg:hidden fixed inset-0 z-50 bg-background/95 overflow-y-auto pt-16">
                <div className="p-4">
                  <div className="flex items-center justify-between mb-6">
                    <h2 className="font-mono text-sm tracking-wider">SIDEBAR</h2>
                    <button
                      onClick={() => setSidebarOpen(false)}
                      className="font-mono text-xs tracking-wider border border-border px-4 py-2 hover:bg-foreground hover:text-background hover:border-foreground transition-colors"
                    >
                      CLOSE
                    </button>
                  </div>
                  <FeedSidebar />
                </div>
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
