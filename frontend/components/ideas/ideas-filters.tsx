"use client";

import { useState, useEffect } from "react";
import { Search, SlidersHorizontal, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface IdeasFilterStats {
  total: number;
  spark: number;
  developing: number;
  mature: number;
  realized: number;
  archived: number;
}

export interface IdeasFiltersProps {
  stats?: IdeasFilterStats;
  onFiltersChange?: (filters: {
    stage: string;
    potential: string;
    sort: string;
    search: string;
    tags: string[];
  }) => void;
}

function getStages(stats?: IdeasFilterStats) {
  return [
    { id: "all", label: "ALL", count: stats?.total ?? 0 },
    { id: "spark", label: "SPARK", count: stats?.spark ?? 0, color: "text-amber-600" },
    { id: "developing", label: "DEVELOPING", count: stats?.developing ?? 0, color: "text-blue-600" },
    { id: "mature", label: "MATURE", count: stats?.mature ?? 0, color: "text-purple-600" },
    { id: "realized", label: "REALIZED", count: stats?.realized ?? 0, color: "text-emerald-600" },
    { id: "archived", label: "ARCHIVED", count: stats?.archived ?? 0, color: "text-muted-foreground" },
  ];
}

const potentialFilters = [
  { id: "any", label: "ANY POTENTIAL" },
  { id: "high", label: "HIGH POTENTIAL" },
  { id: "rising", label: "RISING" },
  { id: "needs-validation", label: "NEEDS VALIDATION" },
];

const sortOptions = [
  { id: "trending", label: "TRENDING" },
  { id: "newest", label: "NEWEST" },
  { id: "most-support", label: "MOST SUPPORT" },
  { id: "most-discussed", label: "MOST DISCUSSED" },
  { id: "ready-to-develop", label: "READY TO DEVELOP" },
];

export function IdeasFilters({ stats, onFiltersChange }: IdeasFiltersProps) {
  const [activeStage, setActiveStage] = useState("all");
  const [showFilters, setShowFilters] = useState(false);
  const [activePotential, setActivePotential] = useState("any");
  const [activeSort, setActiveSort] = useState("trending");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);

  const stages = getStages(stats);

  const availableTags = [
    "architecture", "ai-agents", "collaboration", "performance",
    "ux", "infrastructure", "security", "integrations"
  ];

  // Notify parent when filters change
  useEffect(() => {
    onFiltersChange?.({
      stage: activeStage,
      potential: activePotential,
      sort: activeSort,
      search: searchQuery,
      tags: selectedTags,
    });
  }, [activeStage, activePotential, activeSort, searchQuery, selectedTags, onFiltersChange]);

  const toggleTag = (tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  };

  return (
    <div className="border-b border-border bg-card overflow-hidden">
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        {/* Stage Tabs */}
        <div className="flex items-center gap-1 py-4 overflow-x-auto scrollbar-hide -mx-4 px-4 sm:mx-0 sm:px-0">
          {stages.map((stage) => (
            <button
              key={stage.id}
              onClick={() => setActiveStage(stage.id)}
              className={cn(
                "px-4 py-2 font-mono text-xs tracking-wider transition-colors whitespace-nowrap",
                activeStage === stage.id
                  ? "bg-foreground text-background"
                  : "text-muted-foreground hover:text-foreground hover:bg-secondary"
              )}
            >
              {stage.label}
              <span
                className={cn(
                  "ml-2",
                  activeStage === stage.id ? "text-background/70" : stage.color || "text-muted-foreground"
                )}
              >
                {stage.count}
              </span>
            </button>
          ))}
        </div>

        {/* Search and Filter Toggle */}
        <div className="flex items-center gap-2 sm:gap-4 py-4 border-t border-border">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search ideas..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-secondary/50 border border-border pl-10 pr-4 py-2 font-mono text-sm focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
            />
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setShowFilters(!showFilters)}
            className={cn(
              "font-mono text-xs",
              showFilters && "bg-foreground text-background border-foreground"
            )}
          >
            <SlidersHorizontal className="w-3 h-3 mr-2" />
            FILTERS
          </Button>
        </div>

        {/* Expanded Filters */}
        {showFilters && (
          <div className="py-4 border-t border-border space-y-4">
            {/* Potential Filter */}
            <div>
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground block mb-2">
                POTENTIAL
              </span>
              <div className="flex flex-wrap gap-2">
                {potentialFilters.map((filter) => (
                  <button
                    key={filter.id}
                    onClick={() => setActivePotential(filter.id)}
                    className={cn(
                      "px-3 py-1.5 font-mono text-[10px] tracking-wider border transition-colors",
                      activePotential === filter.id
                        ? "bg-foreground text-background border-foreground"
                        : "border-border text-muted-foreground hover:text-foreground hover:border-foreground/50"
                    )}
                  >
                    {filter.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Sort */}
            <div>
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground block mb-2">
                SORT BY
              </span>
              <div className="flex flex-wrap gap-2">
                {sortOptions.map((option) => (
                  <button
                    key={option.id}
                    onClick={() => setActiveSort(option.id)}
                    className={cn(
                      "px-3 py-1.5 font-mono text-[10px] tracking-wider border transition-colors",
                      activeSort === option.id
                        ? "bg-foreground text-background border-foreground"
                        : "border-border text-muted-foreground hover:text-foreground hover:border-foreground/50"
                    )}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Tags */}
            <div>
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground block mb-2">
                TAGS
              </span>
              <div className="flex flex-wrap gap-2">
                {availableTags.map((tag) => (
                  <button
                    key={tag}
                    onClick={() => toggleTag(tag)}
                    className={cn(
                      "px-3 py-1.5 font-mono text-[10px] tracking-wider border transition-colors",
                      selectedTags.includes(tag)
                        ? "bg-foreground text-background border-foreground"
                        : "border-border text-muted-foreground hover:text-foreground hover:border-foreground/50"
                    )}
                  >
                    {tag}
                    {selectedTags.includes(tag) && <X className="w-2 h-2 ml-1 inline" />}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
