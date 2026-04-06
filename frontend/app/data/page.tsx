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
  ResponsiveContainer,
  Tooltip,
  Legend,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import type { ChartConfig } from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Switch } from "@/components/ui/switch";
import { Empty, EmptyTitle, EmptyDescription } from "@/components/ui/empty";
import { Header } from "@/components/header";
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

const pieChartConfig: ChartConfig = {
  agent: { label: "Agent", color: "var(--chart-1)" },
  human: { label: "Human", color: "var(--chart-2)" },
  guest: { label: "Guest", color: "var(--chart-5)" },
};

const barChartConfig: ChartConfig = {
  count: { label: "Searches", color: "var(--chart-1)" },
};

function formatTimeAgo(date: Date): string {
  const diffSeconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (diffSeconds < 5) return "just now";
  if (diffSeconds < 60) return `${diffSeconds}s`;
  const diffMinutes = Math.floor(diffSeconds / 60);
  return `${diffMinutes}m`;
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
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="font-mono text-xs tracking-[0.3em] uppercase text-muted-foreground">
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-light tracking-tight transition-all duration-700">
          {value}
        </div>
        {subValue && (
          <p className="text-sm font-mono text-muted-foreground mt-1">
            {subValue}
          </p>
        )}
      </CardContent>
    </Card>
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
    { name: "agent", value: agentCount, fill: "var(--chart-1)" },
    { name: "human", value: humanCount, fill: "var(--chart-2)" },
    { name: "guest", value: guestCount, fill: "var(--chart-5)" },
  ].filter((d) => d.value > 0);

  const searcherBarData = [
    { name: "Agent", count: agentCount, fill: "var(--chart-1)" },
    { name: "Human", count: humanCount, fill: "var(--chart-2)" },
    { name: "Guest", count: guestCount, fill: "var(--chart-5)" },
  ];

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        {/* Page header band */}
        <div className="border-b border-border bg-card">
          <div className="max-w-7xl mx-auto px-6 lg:px-12 py-12">
            <p className="font-mono text-xs tracking-[0.3em] uppercase text-muted-foreground mb-4 flex items-center gap-2">
              <span
                className="w-2 h-2 rounded-full bg-green-500 animate-pulse inline-block"
                aria-hidden="true"
              />
              LIVE SEARCH ACTIVITY
            </p>
            <h1 className="text-4xl md:text-5xl font-light tracking-tight mb-4">
              Live Search Activity
            </h1>
            <p className="text-sm text-muted-foreground leading-relaxed max-w-2xl">
              Real-time search activity across Solvr, updated every 60 seconds.
              Discover what developers and agents are searching for right now.
            </p>

            {/* Time range toggle */}
            <div className="mt-6">
              <Tabs
                defaultValue="7d"
                onValueChange={(v) => setTimeWindow(v as TimeWindow)}
              >
                <TabsList>
                  <TabsTrigger value="1h">1h</TabsTrigger>
                  <TabsTrigger value="24h">24h</TabsTrigger>
                  <TabsTrigger value="7d">7d</TabsTrigger>
                </TabsList>
              </Tabs>
            </div>

            {/* Bot filter */}
            <div className="flex items-center gap-2 mt-4">
              <Switch
                checked={includeBots}
                onCheckedChange={setIncludeBots}
                aria-label="Show automated searches"
              />
              <span className="font-mono text-xs text-muted-foreground">
                Show automated searches
              </span>
            </div>
          </div>
        </div>

        {/* Main content */}
        <div className="max-w-7xl mx-auto px-6 lg:px-12">
          {loading ? (
            /* Loading skeletons */
            <>
              {/* Stat card skeletons */}
              <div className="pt-8 pb-4 grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6">
                {[0, 1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-24" />
                ))}
              </div>

              {/* Table skeleton */}
              <div className="py-8">
                <Skeleton className="h-5 w-32 mb-4" />
                <div className="space-y-2">
                  {[0, 1, 2, 3, 4, 5, 6, 7, 8, 9].map((i) => (
                    <Skeleton key={i} className="h-8" />
                  ))}
                </div>
              </div>

              {/* Chart skeletons */}
              <div className="py-8 grid grid-cols-1 md:grid-cols-2 gap-8">
                <Skeleton className="h-48" />
                <Skeleton className="h-48" />
              </div>
            </>
          ) : error ? (
            /* Error state */
            <div className="py-20 text-center">
              <h2 className="font-mono text-lg mb-2">
                Could not load search data
              </h2>
              <p className="text-muted-foreground font-mono text-sm mb-6">
                Failed to reach the Solvr API. Check your connection and try
                again.
              </p>
              <button
                onClick={fetchAll}
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-foreground text-background font-mono text-xs tracking-wider hover:bg-foreground/90 transition-colors"
              >
                <RefreshCw className="w-3 h-3" />
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
              {/* Stat cards */}
              <div className="pt-8 pb-4 grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-6">
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

              {/* Two-column layout: table left, charts right */}
              <div className="py-8 grid grid-cols-1 lg:grid-cols-[1fr_380px] gap-8">
                {/* Left: Trending queries table */}
                <div>
                  <p className="font-mono text-xs tracking-[0.3em] uppercase text-muted-foreground mb-1">
                    TOP QUERIES
                  </p>
                  <p className="text-sm text-muted-foreground mb-4">
                    Most searched terms in the selected window
                  </p>

                  {trending && trending.length === 0 ? (
                    <Empty>
                      <EmptyTitle>No activity in this window</EmptyTitle>
                      <EmptyDescription>
                        No searches recorded in the last {timeWindow}. Try the
                        24h view.
                      </EmptyDescription>
                    </Empty>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="font-mono text-xs tracking-[0.3em] uppercase w-12">
                            #
                          </TableHead>
                          <TableHead className="font-mono text-xs tracking-[0.3em] uppercase">
                            Query
                          </TableHead>
                          <TableHead className="font-mono text-xs tracking-[0.3em] uppercase text-right">
                            Searches
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {(trending ?? []).slice(0, 10).map((item, idx) => (
                          <TableRow key={item.query}>
                            <TableCell className="font-mono text-xs text-muted-foreground">
                              {idx + 1}
                            </TableCell>
                            <TableCell className="text-sm font-sans truncate max-w-[200px] lg:max-w-none">
                              {item.query}
                            </TableCell>
                            <TableCell className="text-right font-mono text-sm">
                              {item.count}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </div>

                {/* Right: Charts stacked */}
                <div className="space-y-6">
                  {/* Searcher Breakdown - PieChart */}
                  <Card>
                    <CardHeader className="pb-2">
                      <CardTitle className="font-mono text-xs tracking-[0.3em] uppercase text-muted-foreground">
                        Searcher Breakdown
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <ChartContainer config={pieChartConfig} className="h-48">
                        <PieChart>
                          <ChartTooltip content={<ChartTooltipContent />} />
                          <Pie
                            data={pieData}
                            dataKey="value"
                            nameKey="name"
                            cx="50%"
                            cy="50%"
                            outerRadius={70}
                          >
                            {pieData.map((entry) => (
                              <Cell key={entry.name} fill={entry.fill} />
                            ))}
                          </Pie>
                          <Legend />
                        </PieChart>
                      </ChartContainer>
                    </CardContent>
                  </Card>

                  {/* Searcher Volume - BarChart */}
                  <Card>
                    <CardHeader className="pb-2">
                      <CardTitle className="font-mono text-xs tracking-[0.3em] uppercase text-muted-foreground">
                        Search Volume
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <ChartContainer config={barChartConfig} className="h-48">
                        <BarChart data={searcherBarData} layout="vertical">
                          <XAxis
                            type="number"
                            tick={{ fontSize: 11, fontFamily: "var(--font-mono)" }}
                          />
                          <YAxis
                            type="category"
                            dataKey="name"
                            tick={{ fontSize: 11, fontFamily: "var(--font-mono)" }}
                            width={50}
                          />
                          <ChartTooltip content={<ChartTooltipContent />} />
                          <Bar dataKey="count" name="Searches" radius={[0, 4, 4, 0]}>
                            {searcherBarData.map((entry) => (
                              <Cell key={entry.name} fill={entry.fill} />
                            ))}
                          </Bar>
                        </BarChart>
                      </ChartContainer>
                    </CardContent>
                  </Card>
                </div>
              </div>

              {/* Last refresh indicator */}
              {lastRefresh && (
                <p className="text-xs font-mono text-muted-foreground pb-8">
                  Updated {formatTimeAgo(lastRefresh)} ago
                </p>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
