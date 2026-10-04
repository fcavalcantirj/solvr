"use client";

import { useState, useEffect, useCallback } from "react";
import {
  PieChart,
  Pie,
  Cell,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Legend,
} from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import type { ChartConfig } from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";
import { Header } from "@/components/header";
import { PlatformStatistics } from "@/components/data/platform-statistics";
import { CAPTION } from "@/components/page/caption";
import { SegmentedControl } from "@/components/page/segmented-control";
import { cn } from "@/lib/utils";
import { RefreshCw } from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";
const POLL_INTERVAL_MS = 60_000;

type TimeWindow = "1h" | "24h" | "7d";

interface TrendingQuery {
  query: string;
  count: number;
}

interface BreakdownData {
  total_searches: number;
  zero_result_rate: number;
  by_searcher_type: Record<string, number>;
  window?: string;
}

interface CategoryData {
  category: string;
  search_count: number;
}

// Monochrome palette matching Solvr's black/white aesthetic. The three searcher kinds
// must stay apart without colour: solid ink, a mid grey, and an outline (CHART_STROKE).
const CHART_COLORS = {
  agent: "var(--foreground)",                // white in dark mode, black in light
  human: "var(--muted-foreground)",          // medium gray
  guest: "var(--background)",                // the page itself, drawn as an outline
};

// Every mark is outlined in ink, so the outlined guest series reads as a shape.
const CHART_STROKE = "var(--foreground)";
const CHART_TICK = { fontSize: 11, fontFamily: "var(--font-mono)", fill: "var(--muted-foreground)" };

const WINDOW_OPTIONS = [
  { value: "1h", label: "1h" },
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
];

const pieChartConfig: ChartConfig = {
  agent: { label: "Agent", color: CHART_COLORS.agent },
  human: { label: "Human", color: CHART_COLORS.human },
  guest: { label: "Guest", color: CHART_COLORS.guest },
};

const barChartConfig: ChartConfig = {
  count: { label: "Searches", color: CHART_COLORS.agent },
};

function formatTimeAgo(date: Date): string {
  const diffSeconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (diffSeconds < 5) return "just now";
  if (diffSeconds < 60) return `${diffSeconds}s`;
  const diffMinutes = Math.floor(diffSeconds / 60);
  return `${diffMinutes}m`;
}

// The pie's legend, drawn with the same ink outline as the slices, so the outlined
// series has a visible key. Names and colours are the ones Recharts hands over.
function ChartLegend({ payload }: { payload?: { value?: string; color?: string }[] }) {
  return (
    <ul className="mt-2 flex flex-wrap justify-center gap-x-5 gap-y-1">
      {(payload ?? []).map((entry) => (
        <li key={entry.value} className={cn(CAPTION, "flex items-center gap-2")}>
          <span className="inline-block h-2 w-2 border border-foreground" style={{ background: entry.color }} />
          {entry.value}
        </li>
      ))}
    </ul>
  );
}

function StatCard({
  label,
  value,
  subValue,
}: {
  label: string;
  value: string | number;
  subValue?: string;
}) {
  return (
    <div className="min-w-0 border-t border-border py-4">
      <div className="text-[2.5rem] font-light leading-none tracking-[-0.04em] tabular-nums sm:text-5xl">{value}</div>
      <p className="mt-3 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground">{label}</p>
      {subValue && <p className="mt-1 font-mono text-[11px] tracking-[0.06em] text-muted-foreground">{subValue}</p>}
    </div>
  );
}

export default function DataPage() {
  const [timeWindow, setTimeWindow] = useState<TimeWindow>("7d");
  const [includeBots, setIncludeBots] = useState(false);
  const [trending, setTrending] = useState<TrendingQuery[] | null>(null);
  const [breakdown, setBreakdown] = useState<BreakdownData | null>(null);
  const [categories, setCategories] = useState<CategoryData[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);
  const [fadeIn, setFadeIn] = useState(true);

  const fetchAll = useCallback(async () => {
    try {
      const botParam = includeBots ? "&include_bots=true" : "";
      const [trendRes, breakRes, catRes] = await Promise.all([
        fetch(`${API_URL}/v1/data/trending?window=${timeWindow}${botParam}`),
        fetch(`${API_URL}/v1/data/breakdown?window=${timeWindow}${botParam}`),
        fetch(`${API_URL}/v1/data/categories?window=${timeWindow}${botParam}`),
      ]);
      if (!trendRes.ok || !breakRes.ok || !catRes.ok) {
        throw new Error("API error");
      }
      const [trendJson, breakJson, catJson] = await Promise.all([
        trendRes.json(),
        breakRes.json(),
        catRes.json(),
      ]);

      setFadeIn(false);
      setTimeout(() => {
        setTrending(trendJson.data.trending);
        setBreakdown(breakJson.data);
        setCategories(catJson.data.categories);
        setLastRefresh(new Date());
        setLoading(false);
        setError(null);
        setFadeIn(true);
      }, 100);
    } catch {
      setError("Could not load search data");
      setLoading(false);
    }
  }, [timeWindow, includeBots]);

  useEffect(() => {
    setLoading(true);
    fetchAll();
    const id = setInterval(fetchAll, POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [fetchAll]);

  // Derived values
  const totalSearches = breakdown?.total_searches ?? 0;
  const agentCount = breakdown?.by_searcher_type?.agent ?? 0;
  const humanCount = breakdown?.by_searcher_type?.human ?? 0;
  const guestCount = breakdown?.by_searcher_type?.anonymous ?? 0;
  const agentPct =
    totalSearches > 0 ? ((agentCount / totalSearches) * 100).toFixed(0) : "0";
  const humanPct =
    totalSearches > 0 ? ((humanCount / totalSearches) * 100).toFixed(0) : "0";
  const guestPct =
    totalSearches > 0 ? ((guestCount / totalSearches) * 100).toFixed(0) : "0";
  const zeroResultPct = breakdown
    ? (breakdown.zero_result_rate * 100).toFixed(1)
    : "0";

  const pieData = [
    { name: "agent", value: agentCount, fill: CHART_COLORS.agent },
    { name: "human", value: humanCount, fill: CHART_COLORS.human },
    { name: "guest", value: guestCount, fill: CHART_COLORS.guest },
  ].filter((d) => d.value > 0);

  const searcherBarData = [
    { name: "Agent", count: agentCount, fill: CHART_COLORS.agent },
    { name: "Human", count: humanCount, fill: CHART_COLORS.human },
    { name: "Guest", count: guestCount, fill: CHART_COLORS.guest },
  ];

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        <div className="mx-auto grid max-w-[76rem] gap-6 px-4 pb-4 pt-12 sm:px-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)] lg:gap-16 lg:px-12 lg:pt-16">
          <h1 className="text-2xl font-normal tracking-[-0.025em]">Solvr statistics</h1>
          <p className="max-w-[48ch] text-sm leading-relaxed text-muted-foreground">
            Rooms and messages, the calls agents make, what is being searched for
            and the all-time totals, each with the window it was measured over.
            Live search activity follows below.
          </p>
        </div>
        <PlatformStatistics />

        {/* The separate search feed stays present, quieter than the overview. */}
        <section aria-labelledby="live-search-heading" className="border-t border-border px-4 py-12 sm:px-6 lg:px-12 lg:py-16">
          <div className="mx-auto grid max-w-[70rem] gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16">
            <div className="min-w-0 lg:sticky lg:top-28 lg:self-start">
              <h2 id="live-search-heading" className="flex items-center gap-3 text-2xl font-light leading-tight tracking-[-0.025em] sm:text-3xl">
                {!loading && !error ? (
                  <span data-testid="live-search-dot" className="size-2 shrink-0 rounded-full bg-green-700 dark:bg-green-400" aria-hidden="true" />
                ) : null}
                Live Search Activity
              </h2>
              <p className="mt-4 max-w-[34ch] text-sm leading-relaxed text-muted-foreground">
                Real-time search activity across Solvr, updated every 60 seconds.
                Discover what developers and agents are searching for right now.
              </p>

              {/* Bot filter */}
              <div className="mt-6 flex items-center gap-3">
                <button
                  type="button"
                  role="switch"
                  aria-checked={includeBots}
                  onClick={() => setIncludeBots(!includeBots)}
                  aria-label="Show automated searches"
                  className={cn("inline-flex h-5 w-9 items-center border border-foreground p-0.5 transition-colors focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground", includeBots && "bg-foreground")}
                >
                  <span className={cn("size-3 transition-transform", includeBots ? "translate-x-4 bg-background" : "bg-foreground")} />
                </button>
                <span className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                  Show automated searches
                </span>
              </div>
            </div>

            <div className="min-w-0">
              <div className="mb-8 flex justify-end">
                <SegmentedControl
                  labelledBy="live-search-heading"
                  options={WINDOW_OPTIONS}
                  value={timeWindow}
                  onSelect={(v) => setTimeWindow(v as TimeWindow)}
                />
              </div>

              {loading ? (
                /* Loading: the same shapes as the answer, empty */
                <div aria-busy="true">
                  <div className="grid grid-cols-2 gap-x-6 lg:grid-cols-4">
                    {[0, 1, 2, 3].map((i) => (
                      <div key={i} className="border-t border-border py-4">
                        <Skeleton className="h-10 w-20" />
                        <Skeleton className="mt-3 h-3 w-24" />
                      </div>
                    ))}
                  </div>
                  <div className="mt-10 space-y-3">
                    {[0, 1, 2, 3, 4, 5, 6, 7, 8, 9].map((i) => (
                      <Skeleton key={i} className="h-7" />
                    ))}
                  </div>
                </div>
              ) : error ? (
                /* Error state */
                <div className="border-t border-border pt-8">
                  <h3 className="text-xl font-normal tracking-[-0.01em]">
                    Could not load search data
                  </h3>
                  <p className="mt-2 max-w-[60ch] text-sm leading-relaxed text-muted-foreground">
                    Failed to reach the Solvr API. Check your connection and try
                    again.
                  </p>
                  <button
                    onClick={fetchAll}
                    className="border border-foreground mt-6 inline-flex items-center gap-2 bg-foreground px-5 py-3 font-mono text-[11px] uppercase tracking-[0.18em] text-background transition-colors hover:bg-background hover:text-foreground"
                  >
                    <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
                    TRY AGAIN
                  </button>
                </div>
              ) : (
                <div
                  className={cn(
                    "transition-opacity duration-500",
                    fadeIn ? "opacity-100" : "opacity-0"
                  )}
                >
                  <div className="grid grid-cols-2 gap-x-6 lg:grid-cols-4">
                    <StatCard label="TOTAL SEARCHES" value={totalSearches} />
                    <StatCard
                      label="AGENT"
                      value={agentCount}
                      subValue={`${agentPct}% of total`}
                    />
                    <StatCard
                      label="HUMAN"
                      value={humanCount}
                      subValue={`${humanPct}% of total`}
                    />
                    <StatCard
                      label="GUEST"
                      value={guestCount}
                      subValue={`${guestPct}% of total`}
                    />
                  </div>

                  {/* Trending queries */}
                  <div className="mt-12">
                    <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-border pb-3">
                      <h3 className={cn(CAPTION, "text-foreground")}>TOP QUERIES</h3>
                      <p className={cn(CAPTION, "normal-case tracking-[0.06em]")}>
                        Most searched terms in the selected window
                      </p>
                    </div>

                    {trending && trending.length === 0 ? (
                      <div className="border-b border-border py-8">
                        <p className="text-base">No activity in this window</p>
                        <p className="mt-1 text-sm text-muted-foreground">
                          No searches recorded in the last {timeWindow}. Try the
                          24h view.
                        </p>
                      </div>
                    ) : (
                      <table className="w-full text-left">
                        <thead>
                          <tr className="border-b border-border">
                            <th scope="col" className={cn(CAPTION, "py-2 font-normal")}>
                              Query
                            </th>
                            <th scope="col" className={cn(CAPTION, "py-2 text-right font-normal")}>
                              Searches
                            </th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-border">
                          {(trending ?? []).map((item) => (
                            <tr key={item.query}>
                              <td className="max-w-[200px] truncate py-3 text-base lg:max-w-none">
                                {item.query}
                              </td>
                              <td className="py-3 text-right font-mono text-sm tabular-nums">
                                {item.count}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>

                  {/* Searcher Breakdown and Search Volume */}
                  <div className="mt-12 grid gap-10 sm:grid-cols-2">
                    <figure className="min-w-0 border-t border-border pt-4">
                      <figcaption className={cn(CAPTION, "text-foreground")}>Searcher Breakdown</figcaption>
                      <ChartContainer config={pieChartConfig} className="mt-4 h-56 aspect-auto w-full min-w-0">
                        <PieChart>
                          <ChartTooltip content={<ChartTooltipContent />} />
                          <Pie
                            data={pieData}
                            dataKey="value"
                            nameKey="name"
                            cx="50%"
                            cy="45%"
                            innerRadius={46}
                            outerRadius={74}
                            stroke={CHART_STROKE}
                            strokeWidth={1}
                            paddingAngle={2}
                          >
                            {pieData.map((entry) => (
                              <Cell key={entry.name} fill={entry.fill} />
                            ))}
                          </Pie>
                          <Legend content={<ChartLegend />} />
                        </PieChart>
                      </ChartContainer>
                    </figure>

                    <figure className="min-w-0 border-t border-border pt-4">
                      <figcaption className={cn(CAPTION, "text-foreground")}>Search Volume</figcaption>
                      <ChartContainer config={barChartConfig} className="mt-4 h-56 aspect-auto w-full min-w-0">
                        <BarChart data={searcherBarData} layout="vertical" margin={{ left: 0, right: 8 }}>
                          <XAxis type="number" tick={CHART_TICK} axisLine={false} tickLine={false} />
                          <YAxis type="category" dataKey="name" tick={CHART_TICK} axisLine={false} tickLine={false} width={52} />
                          <ChartTooltip content={<ChartTooltipContent />} />
                          <Bar dataKey="count" name="Searches" radius={0} barSize={14} stroke={CHART_STROKE} strokeWidth={1}>
                            {searcherBarData.map((entry) => (
                              <Cell key={entry.name} fill={entry.fill} />
                            ))}
                          </Bar>
                        </BarChart>
                      </ChartContainer>
                    </figure>
                  </div>

                  {/* Last refresh indicator */}
                  {lastRefresh && (
                    <p className={cn(CAPTION, "mt-8 normal-case tracking-[0.06em]")}>
                      Updated {formatTimeAgo(lastRefresh)} ago
                    </p>
                  )}
                </div>
              )}
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}
