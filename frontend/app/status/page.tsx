"use client";

export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { Skeleton } from "@/components/ui/skeleton";
import { Caption, CAPTION } from "@/components/page/caption";
import { useStatus } from "@/hooks/use-status";
import { CheckCircle2, XCircle, Server, ChevronDown, ChevronUp, RefreshCw } from "lucide-react";
import { useState } from "react";
import type { APIStatusService, APIStatusIncident, APIStatusUptimeDay } from "@/lib/api-types";

type ServiceStatus = "operational" | "degraded" | "outage";

const SECTION = "grid min-w-0 gap-8 border-t border-border px-4 py-12 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16 lg:px-12 lg:py-16";
const HEADING = "text-2xl font-light leading-tight tracking-[-0.025em] sm:text-3xl";

function StatusBadge({ status }: { status: ServiceStatus }) {
  const config = {
    operational: {
      label: "Operational",
      className: "text-green-700 dark:text-green-400",
      dotClassName: "bg-green-700 dark:bg-green-400",
    },
    degraded: {
      label: "Degraded",
      className: "text-amber-700 dark:text-amber-400",
      dotClassName: "bg-amber-700 dark:bg-amber-400",
    },
    outage: {
      label: "Outage",
      className: "text-red-700 dark:text-red-400",
      dotClassName: "bg-red-700 dark:bg-red-400",
    },
  };
  const { label, className } = config[status];

  return (
    <span className={`inline-flex items-center gap-2 text-xs ${className}`}>
      <span aria-hidden="true" className={`size-1.5 shrink-0 rounded-full ${config[status].dotClassName}`} />
      {label}
    </span>
  );
}

function IncidentStatusBadge({ status }: { status: APIStatusIncident["status"] }) {
  const config = {
    investigating: { label: "Investigating", className: "text-amber-700 dark:text-amber-400" },
    identified: { label: "Identified", className: "text-amber-700 dark:text-amber-400" },
    monitoring: { label: "Monitoring", className: "text-amber-700 dark:text-amber-400" },
    resolved: { label: "Resolved", className: "text-green-700 dark:text-green-400" },
  };
  const { label, className } = config[status];

  return <span className={`inline-flex items-center text-xs ${className}`}>{label}</span>;
}

function ServiceRow({ service }: { service: APIStatusService }) {
  return (
    <div className="grid min-w-0 gap-4 border-b border-border py-6 sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-8">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <h3 className="text-lg font-light [overflow-wrap:anywhere]">{service.name}</h3>
          <StatusBadge status={service.status} />
        </div>
        <p className="mt-2 max-w-[48ch] text-sm leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">{service.description}</p>
        {service.last_checked && (
          <p className="mt-2 text-xs text-muted-foreground">{formatRelativeTime(service.last_checked)}</p>
        )}
      </div>
      <div className="flex flex-wrap items-start gap-x-8 gap-y-3 sm:justify-end sm:text-right">
        <div>
          <span className="block text-3xl font-light tracking-[-0.04em] tabular-nums sm:text-4xl">{service.uptime}</span>
          <Caption className="mt-2">uptime</Caption>
        </div>
        {service.latency_ms != null && (
          <div>
            <span className="block text-3xl font-light tracking-[-0.04em] tabular-nums sm:text-4xl">{service.latency_ms}ms</span>
            <Caption className="mt-2">avg</Caption>
          </div>
        )}
      </div>
    </div>
  );
}

function IncidentRow({ incident }: { incident: APIStatusIncident }) {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <div className="min-w-0 border-t border-border">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        aria-expanded={isExpanded}
        className="flex w-full items-start justify-between gap-4 py-6 text-left focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground"
      >
        <div className="min-w-0 flex-1">
          <h3 className="mb-3 text-xl font-light [overflow-wrap:anywhere]">{incident.title}</h3>
          <div className="mb-2 flex flex-wrap items-center gap-3">
            <Caption as="span">{incident.id}</Caption>
            <IncidentStatusBadge status={incident.status} />
          </div>
          <p className="text-xs leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">
            {incident.created_at} — {incident.updated_at}
          </p>
        </div>
        <div className="mt-1 shrink-0">
          {isExpanded ? (
            <ChevronUp size={16} className="text-muted-foreground" />
          ) : (
            <ChevronDown size={16} className="text-muted-foreground" />
          )}
        </div>
      </button>
      {isExpanded && (
        <div className="space-y-5 border-t border-border py-6">
          {incident.updates.map((update, index) => (
            <div key={index} className="grid grid-cols-[4rem_minmax(0,1fr)] gap-4">
              <div className="pt-0.5 font-mono text-[11px] text-muted-foreground [overflow-wrap:anywhere]">{update.time}</div>
              <p className="text-sm leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">{update.message}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function formatRelativeTime(dateString: string): string {
  const now = new Date();
  const date = new Date(dateString);
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);

  if (diffMin < 1) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHrs = Math.floor(diffMin / 60);
  if (diffHrs < 24) return `${diffHrs}h ago`;
  const diffDays = Math.floor(diffHrs / 24);
  return `${diffDays}d ago`;
}

function LoadingSkeleton() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main aria-busy="true" className="px-4 pb-16 pt-28 sm:px-6 lg:px-12 lg:pt-32">
        <Skeleton className="mb-12 h-6 w-40" />
        <div className="grid gap-12 lg:grid-cols-[minmax(0,1.25fr)_minmax(0,1fr)] lg:gap-16">
          <div className="space-y-4">
            <Skeleton className="h-24 w-4/5" />
            <Skeleton className="h-24 w-full" />
          </div>
          <div>
            <Skeleton className="mb-8 h-28 w-4/5" />
            {[1, 2, 3].map((i) => (
              <div key={i} className="flex justify-between border-t border-border py-5">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-8 w-20" />
              </div>
            ))}
          </div>
        </div>
        <Skeleton className="mt-16 h-20 w-full" />
      </main>
      <Footer />
    </div>
  );
}

function ErrorState({ error, onRetry }: { error: string; onRetry: () => void }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="px-4 pb-16 pt-32 sm:px-6 lg:px-12">
        <h1 className="flex items-center gap-4 text-3xl font-light sm:text-5xl">
          <XCircle size={32} strokeWidth={1} className="shrink-0 text-red-700 dark:text-red-400" />
          Unable to Load Status
        </h1>
        <p className="mb-8 mt-6 text-sm text-muted-foreground">{error}</p>
        <button onClick={onRetry} className="inline-flex items-center gap-2 border border-border px-5 py-3 text-sm transition-colors hover:bg-secondary focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground">
          <RefreshCw size={14} />
          Retry
        </button>
      </main>
      <Footer />
    </div>
  );
}

function UptimeChart({ history }: { history: APIStatusUptimeDay[] }) {
  const days = history.length > 0 ? history : [];

  return (
    <section className={SECTION}>
      <h2 className={HEADING}>30-DAY UPTIME HISTORY</h2>
      <div className="min-w-0">
        <div className="flex min-w-0 gap-[3px]">
          {days.length > 0 ? (
            days.map((day, index) => (
              // One slot per day of the 30-day window, so a bar always reads as one day wide.
              <div key={index} className="relative w-[calc((100%-87px)/30)] flex-none pb-7">
                <div
                  className={`h-16 w-full border border-foreground sm:h-20 ${
                    day.status === "operational"
                      ? "bg-foreground"
                      : day.status === "degraded"
                        ? "bg-muted-foreground"
                        : "bg-background"
                  }`}
                  title={`${day.date}: ${day.status}`}
                />
                {index === 0 && (
                  <span className="absolute bottom-0 left-0 font-mono text-[11px] text-muted-foreground">Today</span>
                )}
                {index === days.length - 1 && days.length > 1 && (
                  <span className="absolute bottom-0 right-0 font-mono text-[11px] text-muted-foreground">{days.length}d</span>
                )}
              </div>
            ))
          ) : (
            <p className="w-full border-t border-border py-6 text-sm text-muted-foreground">
              Uptime history will appear as health checks accumulate
            </p>
          )}
        </div>
        <div className="mt-6 flex flex-wrap gap-x-6 gap-y-3 text-xs text-muted-foreground">
          <span className="flex items-center gap-2"><span className="size-2 border border-foreground bg-foreground" /> Operational</span>
          <span className="flex items-center gap-2"><span className="size-2 border border-foreground bg-muted-foreground" /> Degraded</span>
          <span className="flex items-center gap-2"><span className="size-2 border border-foreground bg-background" /> Outage</span>
        </div>
      </div>
    </section>
  );
}

export default function StatusPage() {
  const { data, loading, error, refetch } = useStatus();

  if (loading && !data) return <LoadingSkeleton />;
  if (error && !data) return <ErrorState error={error} onRetry={refetch} />;

  const allOperational = data?.overall_status === "operational";

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-16">
        <section className="px-4 pb-12 pt-12 sm:px-6 lg:px-12 lg:pb-16 lg:pt-16">
          <h1 className="text-2xl font-normal tracking-[-0.025em]">Solvr Status</h1>
          <div className="mt-10 grid min-w-0 gap-10 lg:mt-14 lg:grid-cols-[minmax(0,1.25fr)_minmax(0,1fr)] lg:gap-16">
            <div className="min-w-0 lg:border-r lg:border-border lg:pr-12">
              <p className="max-w-[12ch] text-[clamp(2.75rem,7.5vw,8.5rem)] font-light leading-[0.98] tracking-[-0.055em] [overflow-wrap:anywhere]">
                {allOperational ? "All Systems Operational" : "Partial Service Disruption"}
              </p>
              <Caption className="mt-7 flex items-center gap-3">
                <span aria-hidden="true" className={`size-2 shrink-0 rounded-full ${allOperational ? "bg-green-700 dark:bg-green-400" : "bg-amber-700 dark:bg-amber-400"}`} />
                SYSTEM STATUS
              </Caption>
            </div>
            <dl className="min-w-0">
              <div className="flex flex-col pb-8">
                <dd className="text-[clamp(4.5rem,8vw,9rem)] font-light leading-none tracking-[-0.065em] tabular-nums [overflow-wrap:anywhere]">
                  {data?.summary.uptime_30d != null ? `${data.summary.uptime_30d.toFixed(2)}%` : "—"}
                </dd>
                <Caption as="dt" className="mt-4">Overall Uptime (30d)</Caption>
              </div>
              <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] items-center gap-5 border-t border-border py-5">
                <Caption as="dt">Avg Response Time</Caption>
                <dd className="text-right text-4xl font-light leading-none tracking-[-0.04em] tabular-nums [overflow-wrap:anywhere]">
                  {data?.summary.avg_response_time_ms != null
                  ? `${Math.round(data.summary.avg_response_time_ms)}ms`
                  : "—"}
                </dd>
              </div>
              <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] items-center gap-5 border-t border-border py-5">
                <Caption as="dt">Active Services</Caption>
                <dd className="text-right text-4xl font-light leading-none tracking-[-0.04em] tabular-nums [overflow-wrap:anywhere]">{data?.summary.service_count ?? 0}</dd>
              </div>
              <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] items-center gap-5 border-t border-border py-5">
                <Caption as="dt">Last Checked</Caption>
                <dd className="text-right text-3xl font-light leading-none tracking-[-0.04em] [overflow-wrap:anywhere]">
                  {data?.summary.last_checked
                  ? formatRelativeTime(data.summary.last_checked)
                  : "—"}
                </dd>
              </div>
            </dl>
          </div>
        </section>

        <UptimeChart history={data?.uptime_history ?? []} />

        <section className={SECTION}>
          <h2 className={HEADING}>SERVICE STATUS</h2>
          <div className="min-w-0 space-y-10">
            {(data?.services ?? []).map((category) => (
              <div key={category.category} className="min-w-0">
                <div className="flex flex-wrap items-baseline justify-between gap-3 border-b border-border pb-4">
                  <h3 className="text-xl font-light">{category.category}</h3>
                  <span className="text-xs text-muted-foreground">
                    {category.items.filter((s) => s.status === "operational").length}/{category.items.length} operational
                  </span>
                </div>
                {category.items.map((service) => <ServiceRow key={service.name} service={service} />)}
              </div>
            ))}
            {(data?.services ?? []).length === 0 && (
              <div className="border-t border-border py-6">
                <p className="mb-2 flex items-center gap-3 text-xl font-light"><Server size={24} strokeWidth={1} />No service data yet</p>
                <p className="text-sm text-muted-foreground">Health checks will appear after the first monitoring cycle</p>
              </div>
            )}
          </div>
        </section>

        <section className={SECTION}>
          <h2 className={HEADING}>RECENT INCIDENTS</h2>
          <div className="min-w-0">
            {(data?.incidents ?? []).map((incident) => <IncidentRow key={incident.id} incident={incident} />)}
            {(data?.incidents ?? []).length === 0 && (
              <div className="border-t border-border py-6">
                <p className="mb-3 flex items-center gap-3 text-xl font-light sm:text-2xl">
                  <CheckCircle2 size={24} strokeWidth={1} className="shrink-0 text-green-700 dark:text-green-400" />
                  No recent incidents
                </p>
                <p className="text-sm text-muted-foreground">All systems have been operating normally</p>
              </div>
            )}
          </div>
        </section>

        <section className={SECTION}>
          <h2 className={HEADING}>PROGRAMMATIC ACCESS</h2>
          <details className="group min-w-0 border-t border-border">
            <summary className="flex cursor-pointer list-none items-center justify-between gap-4 py-5 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground [&::-webkit-details-marker]:hidden">
              <h3 className="text-xl font-light">Status API</h3>
              <ChevronDown aria-hidden="true" size={16} className="transition-transform group-open:rotate-180" />
            </summary>
            <p className="mb-5 text-sm text-muted-foreground">Get real-time status updates via our JSON API.</p>
            <code className={`${CAPTION} block whitespace-pre-wrap border-t border-border py-5 normal-case tracking-normal [overflow-wrap:anywhere]`}>
              GET https://api.solvr.dev/v1/status
            </code>
          </details>
        </section>
      </main>
      <Footer />
    </div>
  );
}
