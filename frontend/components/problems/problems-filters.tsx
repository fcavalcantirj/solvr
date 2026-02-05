"use client";

import { useState } from "react";
import { Search, SlidersHorizontal, X } from "lucide-react";

const statuses = [
  { key: "all", label: "ALL" },
  { key: "open", label: "OPEN" },
  { key: "in_progress", label: "IN PROGRESS" },
  { key: "stuck", label: "STUCK" },
  { key: "solved", label: "SOLVED" },
];

const weights = [
  { key: "all", label: "ALL" },
  { key: "critical", label: "CRITICAL" },
  { key: "high", label: "HIGH" },
  { key: "medium", label: "MEDIUM" },
  { key: "low", label: "LOW" },
];

const sorts = [
  { key: "newest", label: "NEWEST" },
  { key: "votes", label: "MOST VOTED" },
  { key: "approaches", label: "MOST APPROACHES" },
  { key: "stuck", label: "NEEDS HELP" },
  { key: "activity", label: "RECENT ACTIVITY" },
];

const popularTags = [
  "async",
  "react",
  "typescript",
  "postgresql",
  "node.js",
  "performance",
  "memory",
  "concurrency",
];

export function ProblemsFilters() {
  const [showFilters, setShowFilters] = useState(false);
  const [activeStatus, setActiveStatus] = useState("all");
  const [activeWeight, setActiveWeight] = useState("all");
  const [activeSort, setActiveSort] = useState("newest");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState("");

  const toggleTag = (tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  };

  const clearFilters = () => {
    setActiveStatus("all");
    setActiveWeight("all");
    setActiveSort("newest");
    setSelectedTags([]);
    setSearchQuery("");
  };

  const hasActiveFilters =
    activeStatus !== "all" ||
    activeWeight !== "all" ||
    selectedTags.length > 0 ||
    searchQuery !== "";

  return (
    <div className="border-b border-border bg-card">
      <div className="max-w-7xl mx-auto px-6 lg:px-12">
        {/* Main Filter Row */}
        <div className="flex items-center justify-between py-4 gap-4">
          {/* Search */}
          <div className="flex-1 max-w-lg">
            <div className="relative">
              <Search
                size={16}
                className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-foreground"
              />
              <input
                type="text"
                placeholder="Search problems..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full bg-background border border-border pl-11 pr-4 py-2.5 font-mono text-sm placeholder:text-muted-foreground focus:outline-none focus:border-foreground transition-colors"
              />
            </div>
          </div>

          {/* Status Pills - Desktop */}
          <div className="hidden lg:flex items-center gap-1">
            {statuses.map((status) => (
              <button
                key={status.key}
                onClick={() => setActiveStatus(status.key)}
                className={`font-mono text-[10px] tracking-wider px-3 py-2 transition-colors ${
                  activeStatus === status.key
                    ? "bg-foreground text-background"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {status.label}
              </button>
            ))}
          </div>

          {/* Filter Toggle */}
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={`flex items-center gap-2 font-mono text-[10px] tracking-wider px-4 py-2 border transition-colors ${
              showFilters
                ? "bg-foreground text-background border-foreground"
                : "border-border hover:border-foreground"
            }`}
          >
            <SlidersHorizontal size={12} />
            <span className="hidden sm:inline">FILTERS</span>
          </button>
        </div>

        {/* Status Pills - Mobile */}
        <div className="lg:hidden flex items-center gap-1 pb-4 overflow-x-auto scrollbar-hide">
          {statuses.map((status) => (
            <button
              key={status.key}
              onClick={() => setActiveStatus(status.key)}
              className={`font-mono text-[10px] tracking-wider px-3 py-2 whitespace-nowrap transition-colors ${
                activeStatus === status.key
                  ? "bg-foreground text-background"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {status.label}
            </button>
          ))}
        </div>

        {/* Expanded Filters */}
        {showFilters && (
          <div className="border-t border-border py-5 space-y-5">
            {/* Weight Filter */}
            <div className="flex flex-wrap items-center gap-3">
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground w-16">
                WEIGHT
              </span>
              <div className="flex flex-wrap items-center gap-1">
                {weights.map((weight) => (
                  <button
                    key={weight.key}
                    onClick={() => setActiveWeight(weight.key)}
                    className={`font-mono text-[10px] tracking-wider px-3 py-1.5 transition-colors ${
                      activeWeight === weight.key
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {weight.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Sort Filter */}
            <div className="flex flex-wrap items-center gap-3">
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground w-16">
                SORT
              </span>
              <div className="flex flex-wrap items-center gap-1">
                {sorts.map((sort) => (
                  <button
                    key={sort.key}
                    onClick={() => setActiveSort(sort.key)}
                    className={`font-mono text-[10px] tracking-wider px-3 py-1.5 transition-colors ${
                      activeSort === sort.key
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {sort.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Tags Filter */}
            <div className="flex flex-wrap items-start gap-3">
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground w-16 pt-1.5">
                TAGS
              </span>
              <div className="flex-1 flex flex-wrap items-center gap-1.5">
                {popularTags.map((tag) => (
                  <button
                    key={tag}
                    onClick={() => toggleTag(tag)}
                    className={`font-mono text-[10px] tracking-wider px-3 py-1.5 transition-colors ${
                      selectedTags.includes(tag)
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {tag}
                  </button>
                ))}
              </div>
            </div>

            {/* Active Filters & Clear */}
            {hasActiveFilters && (
              <div className="flex items-center justify-between pt-2 border-t border-border">
                <div className="flex items-center gap-2 flex-wrap">
                  {activeStatus !== "all" && (
                    <span className="font-mono text-[10px] tracking-wider bg-foreground text-background px-2 py-1 flex items-center gap-1.5">
                      {statuses.find((s) => s.key === activeStatus)?.label}
                      <X
                        size={10}
                        className="cursor-pointer"
                        onClick={() => setActiveStatus("all")}
                      />
                    </span>
                  )}
                  {activeWeight !== "all" && (
                    <span className="font-mono text-[10px] tracking-wider bg-foreground text-background px-2 py-1 flex items-center gap-1.5">
                      {weights.find((w) => w.key === activeWeight)?.label}
                      <X
                        size={10}
                        className="cursor-pointer"
                        onClick={() => setActiveWeight("all")}
                      />
                    </span>
                  )}
                  {selectedTags.map((tag) => (
                    <span
                      key={tag}
                      className="font-mono text-[10px] tracking-wider bg-foreground text-background px-2 py-1 flex items-center gap-1.5"
                    >
                      {tag}
                      <X
                        size={10}
                        className="cursor-pointer"
                        onClick={() => toggleTag(tag)}
                      />
                    </span>
                  ))}
                </div>
                <button
                  onClick={clearFilters}
                  className="font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground transition-colors"
                >
                  CLEAR ALL
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
