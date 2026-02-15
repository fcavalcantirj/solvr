"use client";

import { useState } from "react";
import {
  Search,
  SlidersHorizontal,
  X,
  LayoutGrid,
  List,
} from "lucide-react";
import { PostType } from "@/hooks/use-posts";

const types = [
  { label: "All", value: "all" },
  { label: "Problems", value: "problem" },
  { label: "Questions", value: "question" },
  { label: "Ideas", value: "idea" },
] as const;

const statuses = ["All", "Open", "In Progress", "Solved", "Stuck"];
const sorts = ["Newest", "Trending", "Most Voted", "Needs Help"];
const timeframes = ["All Time", "Today", "This Week", "This Month"];

export interface FilterState {
  type: PostType | "all";
  status: string;
  sort: string;
  timeframe: string;
  searchQuery: string;
  viewMode: "list" | "grid";
}

interface FeedFiltersProps {
  filters: FilterState;
  onFiltersChange: (filters: Partial<FilterState>) => void;
  showFilters: boolean;
  onToggleFilters: () => void;
  onToggleSidebar?: () => void;
}

export function FeedFilters({
  filters,
  onFiltersChange,
  showFilters,
  onToggleFilters,
  onToggleSidebar,
}: FeedFiltersProps) {
  const [showMobileSearch, setShowMobileSearch] = useState(false);

  const activeFiltersCount =
    (filters.status !== "All" ? 1 : 0) +
    (filters.timeframe !== "All Time" ? 1 : 0) +
    (filters.searchQuery ? 1 : 0);

  return (
    <div className="border-b border-border bg-card sticky top-16 z-30">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12">
        {/* Main Filter Bar */}
        <div className="flex items-center gap-2 sm:gap-4 py-3 sm:py-4">
          {/* Search - Desktop */}
          <div className="hidden sm:block flex-1 max-w-sm">
            <div className="relative">
              <Search
                size={16}
                className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-foreground"
              />
              <input
                type="text"
                placeholder="Search feed..."
                value={filters.searchQuery}
                onChange={(e) => onFiltersChange({ searchQuery: e.target.value })}
                className="w-full bg-background border border-border pl-11 pr-4 py-2.5 font-mono text-sm placeholder:text-muted-foreground focus:outline-none focus:border-foreground transition-colors"
              />
              {filters.searchQuery && (
                <button
                  onClick={() => onFiltersChange({ searchQuery: "" })}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  <X size={14} />
                </button>
              )}
            </div>
          </div>

          {/* Search Toggle - Mobile */}
          <button
            onClick={() => setShowMobileSearch((prev) => !prev)}
            className="sm:hidden w-10 h-10 flex items-center justify-center border border-border hover:bg-secondary transition-colors"
          >
            <Search size={16} />
          </button>

          {/* Type Tabs - Desktop */}
          <div className="hidden md:flex items-center border border-border">
            {types.map((type) => (
              <button
                key={type.value}
                onClick={() => onFiltersChange({ type: type.value })}
                className={`font-mono text-[11px] tracking-wider px-4 py-2.5 transition-colors border-r border-border last:border-r-0 ${
                  filters.type === type.value
                    ? "bg-foreground text-background"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
                }`}
              >
                {type.label.toUpperCase()}
              </button>
            ))}
          </div>

          {/* Spacer */}
          <div className="flex-1 sm:hidden" />

          {/* View Mode Toggle */}
          <div className="hidden sm:flex items-center border border-border">
            <button
              onClick={() => onFiltersChange({ viewMode: "list" })}
              className={`w-10 h-10 flex items-center justify-center transition-colors ${
                filters.viewMode === "list"
                  ? "bg-foreground text-background"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <List size={16} />
            </button>
            <button
              onClick={() => onFiltersChange({ viewMode: "grid" })}
              className={`w-10 h-10 flex items-center justify-center border-l border-border transition-colors ${
                filters.viewMode === "grid"
                  ? "bg-foreground text-background"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <LayoutGrid size={16} />
            </button>
          </div>

          {/* Filter Toggle */}
          <button
            onClick={onToggleFilters}
            className={`flex items-center gap-2 font-mono text-[11px] tracking-wider px-3 sm:px-4 py-2.5 border transition-colors relative ${
              showFilters
                ? "bg-foreground text-background border-foreground"
                : "border-border hover:border-foreground"
            }`}
          >
            <SlidersHorizontal size={14} />
            <span className="hidden sm:inline">FILTERS</span>
            {activeFiltersCount > 0 && (
              <span className="absolute -top-1.5 -right-1.5 w-5 h-5 bg-foreground text-background text-[10px] flex items-center justify-center">
                {activeFiltersCount}
              </span>
            )}
          </button>

          {/* Sidebar Toggle - Mobile */}
          {onToggleSidebar && (
            <button
              onClick={onToggleSidebar}
              className="lg:hidden font-mono text-[11px] tracking-wider px-3 py-2.5 border border-border hover:border-foreground transition-colors"
            >
              MORE
            </button>
          )}
        </div>

        {/* Mobile Search - Expanded */}
        {showMobileSearch && (
          <div className="sm:hidden pb-3">
            <div className="relative">
              <Search
                size={16}
                className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-foreground"
              />
              <input
                type="text"
                placeholder="Search feed..."
                value={filters.searchQuery}
                onChange={(e) => onFiltersChange({ searchQuery: e.target.value })}
                autoFocus
                className="w-full bg-background border border-border pl-11 pr-4 py-2.5 font-mono text-sm placeholder:text-muted-foreground focus:outline-none focus:border-foreground transition-colors"
              />
              {filters.searchQuery && (
                <button
                  onClick={() => onFiltersChange({ searchQuery: "" })}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  <X size={14} />
                </button>
              )}
            </div>
          </div>
        )}

        {/* Type Tabs - Mobile Horizontal Scroll */}
        <div className="md:hidden -mx-4 sm:-mx-6 px-4 sm:px-6 pb-3 overflow-x-auto scrollbar-hide">
          <div className="flex items-center gap-1 min-w-max">
            {types.map((type) => (
              <button
                key={type.value}
                onClick={() => onFiltersChange({ type: type.value })}
                className={`font-mono text-[10px] tracking-wider px-3 py-2 whitespace-nowrap transition-colors ${
                  filters.type === type.value
                    ? "bg-foreground text-background"
                    : "bg-secondary text-muted-foreground"
                }`}
              >
                {type.label.toUpperCase()}
              </button>
            ))}
          </div>
        </div>

        {/* Expanded Filters */}
        {showFilters && (
          <div className="border-t border-border py-4 space-y-4">
            {/* Status Filter */}
            <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground w-16 flex-shrink-0">
                STATUS
              </span>
              <div className="flex flex-wrap items-center gap-1">
                {statuses.map((status) => (
                  <button
                    key={status}
                    onClick={() => onFiltersChange({ status })}
                    className={`font-mono text-[10px] tracking-wider px-3 py-1.5 transition-colors ${
                      filters.status === status
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {status.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>

            {/* Sort Filter */}
            <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground w-16 flex-shrink-0">
                SORT
              </span>
              <div className="flex flex-wrap items-center gap-1">
                {sorts.map((sort) => (
                  <button
                    key={sort}
                    onClick={() => onFiltersChange({ sort })}
                    className={`font-mono text-[10px] tracking-wider px-3 py-1.5 transition-colors ${
                      filters.sort === sort
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {sort.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>

            {/* Timeframe Filter */}
            <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground w-16 flex-shrink-0">
                TIME
              </span>
              <div className="flex flex-wrap items-center gap-1">
                {timeframes.map((timeframe) => (
                  <button
                    key={timeframe}
                    onClick={() => onFiltersChange({ timeframe })}
                    className={`font-mono text-[10px] tracking-wider px-3 py-1.5 transition-colors ${
                      filters.timeframe === timeframe
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {timeframe.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>

            {/* Clear Filters */}
            {activeFiltersCount > 0 && (
              <div className="pt-2">
                <button
                  onClick={() => onFiltersChange({
                    status: "All",
                    timeframe: "All Time",
                    searchQuery: "",
                  })}
                  className="font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground underline underline-offset-4 transition-colors"
                >
                  CLEAR ALL FILTERS
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
