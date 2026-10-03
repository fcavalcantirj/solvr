"use client";

import { HeroSection } from '@/components/hero-section';
import { useHomepageOverview } from '@/hooks/use-homepage-overview';
import type { APIOverviewResponse } from '@/lib/api-types';
import { LiveOverview } from './live-overview';

// The index below the header: the hero, then the live overview. This is the one
// place the index reads GET /v1/overview in the browser. It starts from what the
// server read (`initial`, so the first HTML already carries the hero numbers) and
// refreshes once; the hero and the sections both follow the refreshed answer, so
// a page served from a cache does not keep showing old numbers. A failed server
// read (`initial` null) degrades to the browser read alone.
export function HomeOverview({ initial }: { initial: APIOverviewResponse | null }) {
  const { overview, meta, loading, error } = useHomepageOverview(initial);

  return (
    <>
      <HeroSection heroNumbers={overview?.hero_numbers} />
      <LiveOverview overview={overview} meta={meta} loading={loading} error={error} />
    </>
  );
}
