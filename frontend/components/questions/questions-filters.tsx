"use client";

import { useState } from "react";
import { Search, SlidersHorizontal, X } from "lucide-react";

const statuses = [
  { key: "all", label: "ALL", apiValue: undefined },
  { key: "unanswered", label: "UNANSWERED", apiValue: "open" },
  { key: "answered", label: "ANSWERED", apiValue: "answered" },
  { key: "accepted", label: "ACCEPTED", apiValue: "solved" },
];

const sorts: Array<{ key: 'newest' | 'votes' | 'answers'; label: string }> = [
  { key: "newest", label: "NEWEST" },
  { key: "votes", label: "MOST VOTED" },
  { key: "answers", label: "MOST ANSWERS" },
];

const popularTags = [
  "javascript",
  "typescript",
  "react",
  "node.js",
  "api-design",
  "database",
  "devops",
  "testing",
];

interface QuestionsFiltersProps {
  status?: string;
  sort: 'newest' | 'votes' | 'answers';
  tags: string[];
  searchQuery: string;
  onStatusChange: (status: string | undefined) => void;
  onSortChange: (sort: 'newest' | 'votes' | 'answers') => void;
  onTagsChange: (tags: string[]) => void;
  onSearchQueryChange: (query: string) => void;
}

export function QuestionsFilters({
  status,
  sort,
  tags,
  searchQuery,
  onStatusChange,
  onSortChange,
  onTagsChange,
  onSearchQueryChange,
}: QuestionsFiltersProps) {
  const [showFilters, setShowFilters] = useState(false);

  // Find active status key from API value
  const activeStatusKey = statuses.find((s) => s.apiValue === status)?.key || "all";

  const handleStatusChange = (key: string) => {
    const selected = statuses.find((s) => s.key === key);
    onStatusChange(selected?.apiValue);
  };

  const handleSortChange = (key: 'newest' | 'votes' | 'answers') => {
    onSortChange(key);
  };

  const toggleTag = (tag: string) => {
    if (tags.includes(tag)) {
      onTagsChange(tags.filter((t) => t !== tag));
    } else {
      onTagsChange([...tags, tag]);
    }
  };

  const clearFilters = () => {
    onStatusChange(undefined);
    onSortChange("newest");
    onTagsChange([]);
    onSearchQueryChange("");
  };

  const hasActiveFilters =
    activeStatusKey !== "all" ||
    tags.length > 0 ||
    searchQuery !== "";

  return (
    <div className="border-b border-border bg-card">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12">
        {/* Main Filter Row */}
        <div className="flex items-center justify-between py-4 gap-4">
          {/* Search */}
          <div className="flex-1 max-w-lg">
            <div className="relative">
              <button
                data-testid="search-icon-button"
                onClick={() => {
                  // Icon click doesn't need to do anything special
                  // Search is already triggered by onChange
                }}
                className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground cursor-pointer transition-colors"
                aria-label="Search"
              >
                <Search size={16} />
              </button>
              <input
                type="text"
                placeholder="Search questions..."
                value={searchQuery}
                onChange={(e) => onSearchQueryChange(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    // Enter key submits current search query
                    // Search is already triggered by onChange, this is just for UX
                  }
                }}
                className="w-full bg-background border border-border pl-11 pr-4 py-2.5 font-mono text-sm placeholder:text-muted-foreground focus:outline-none focus:border-foreground transition-colors"
              />
            </div>
          </div>

          {/* Status Pills - Desktop */}
          <div className="hidden lg:flex items-center gap-1">
            {statuses.map((s) => (
              <button
                key={s.key}
                onClick={() => handleStatusChange(s.key)}
                className={`font-mono text-[10px] tracking-wider px-3 py-2 transition-colors ${
                  activeStatusKey === s.key
                    ? "bg-foreground text-background"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {s.label}
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
          {statuses.map((s) => (
            <button
              key={s.key}
              onClick={() => handleStatusChange(s.key)}
              className={`font-mono text-[10px] tracking-wider px-3 py-2 whitespace-nowrap transition-colors ${
                activeStatusKey === s.key
                  ? "bg-foreground text-background"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {s.label}
            </button>
          ))}
        </div>

        {/* Expanded Filters */}
        {showFilters && (
          <div className="border-t border-border py-5 space-y-5">
            {/* Sort Filter */}
            <div className="flex flex-wrap items-center gap-3">
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground w-16">
                SORT
              </span>
              <div className="flex flex-wrap items-center gap-1">
                {sorts.map((s) => (
                  <button
                    key={s.key}
                    onClick={() => handleSortChange(s.key)}
                    className={`font-mono text-[10px] tracking-wider px-3 py-1.5 transition-colors ${
                      sort === s.key
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {s.label}
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
                      tags.includes(tag)
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
                  {activeStatusKey !== "all" && (
                    <span className="font-mono text-[10px] tracking-wider bg-foreground text-background px-2 py-1 flex items-center gap-1.5">
                      {statuses.find((s) => s.key === activeStatusKey)?.label}
                      <X
                        size={10}
                        className="cursor-pointer"
                        onClick={() => handleStatusChange("all")}
                      />
                    </span>
                  )}
                  {tags.map((tag) => (
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
