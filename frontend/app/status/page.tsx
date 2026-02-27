"use client";

// Force dynamic rendering - this page uses client-side state (useState)
// and should not be statically generated at build time
export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { Skeleton } from "@/components/ui/skeleton";
import { useStatus } from "@/hooks/use-status";
import {
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Activity,
  Database,
  Server,
  Clock,
  ArrowUpRight,
  ChevronDown,
  ChevronUp,
  RefreshCw,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import type {
  APIStatusService,
  APIStatusIncident,
  APIStatusUptimeDay,
} from "@/lib/api-types";

type ServiceStatus = "operational" | "degraded" | "outage";

function StatusBadge({ status }: { status: ServiceStatus }) {
  const config = {
    operational: {
      icon: CheckCircle2,
      label: "Operational",
      className: "text-emerald-600 bg-emerald-50",
      dotClassName: "bg-emerald-500",
    },
    degraded: {
      icon: AlertTriangle,
      label: "Degraded",
      className: "text-amber-600 bg-amber-50",
      dotClassName: "bg-amber-500",
    },
    outage: {
      icon: XCircle,
      label: "Outage",
      className: "text-red-600 bg-red-50",
      dotClassName: "bg-red-500",
    },
  };

  const { label, className } = config[status];

  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-1 text-xs font-mono ${className}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${config[status].dotClassName}`} />
      {label}
    </span>
  );
}

function IncidentStatusBadge({ status }: { status: APIStatusIncident["status"] }) {
  const config = {
    investigating: { label: "Investigating", className: "text-amber-600 bg-amber-50" },
    identified: { label: "Identified", className: "text-blue-600 bg-blue-50" },
    monitoring: { label: "Monitoring", className: "text-indigo-600 bg-indigo-50" },
    resolved: { label: "Resolved", className: "text-emerald-600 bg-emerald-50" },
  };

  const { label, className } = config[status];

  return (
    <span className={`inline-flex items-center px-2 py-1 text-xs font-mono ${className}`}>
      {label}
    </span>
  );
}

function ServiceCard({ service }: { service: APIStatusService }) {
  return (
    <div className="flex items-center justify-between py-4 border-b border-border last:border-0">
      <div className="flex-1 min-w-0 pr-4">
        <div className="flex items-center gap-3 mb-1">
          <h3 className="font-mono text-sm">{service.name}</h3>
          <StatusBadge status={service.status} />
        </div>
        <p className="text-xs text-muted-foreground">{service.description}</p>
      </div>
      <div className="flex items-center gap-6 shrink-0 text-xs text-muted-foreground font-mono">
        <div className="hidden sm:block">
          <span className="text-foreground">{service.uptime}</span> uptime
        </div>
        {service.latency_ms != null && (
          <div className="hidden md:block">
            <span className="text-foreground">{service.latency_ms}ms</span> avg
          </div>
        )}
        {service.last_checked && (
          <div className="text-[10px]">{formatRelativeTime(service.last_checked)}</div>
        )}
      </div>
    </div>
  );
}

function IncidentCard({ incident }: { incident: APIStatusIncident }) {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <div className="border border-border">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full p-4 sm:p-6 text-left flex items-start justify-between gap-4"
      >
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-2 sm:gap-3 mb-2">
            <span className="font-mono text-[10px] text-muted-foreground">{incident.id}</span>
            <IncidentStatusBadge status={incident.status} />
          </div>
          <h3 className="font-mono text-sm mb-2">{incident.title}</h3>
          <p className="text-xs text-muted-foreground">
            {incident.created_at} — {incident.updated_at}
          </p>
        </div>
        <div className="shrink-0 mt-1">
          {isExpanded ? (
            <ChevronUp size={16} className="text-muted-foreground" />
          ) : (
            <ChevronDown size={16} className="text-muted-foreground" />
          )}
        </div>
      </button>

      {isExpanded && (
        <div className="px-4 sm:px-6 pb-6 border-t border-border pt-4">
          <div className="space-y-4">
            {incident.updates.map((update, index) => (
              <div key={index} className="flex gap-4">
                <div className="w-16 shrink-0 font-mono text-[10px] text-muted-foreground pt-0.5">
                  {update.time}
                </div>
                <div className="flex-1">
                  <p className="text-sm text-muted-foreground leading-relaxed">{update.message}</p>
                </div>
              </div>
            ))}
          </div>
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

function getCategoryIcon(category: string) {
  switch (category) {
    case "Core Services":
      return <Server size={16} strokeWidth={1.5} />;
    case "Storage":
      return <Database size={16} strokeWidth={1.5} />;
    default:
      return <Server size={16} strokeWidth={1.5} />;
  }
}

function LoadingSkeleton() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <section className="pt-32 pb-16 px-4 sm:px-6 lg:px-12">
        <div className="max-w-5xl mx-auto">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-6 mb-12">
            <div>
              <Skeleton className="h-3 w-32 mb-4" />
              <Skeleton className="h-10 w-48" />
            </div>
            <Skeleton className="h-12 w-56" />
          </div>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-px bg-border border border-border mb-16">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="bg-background p-4 sm:p-6">
                <Skeleton className="h-4 w-4 mb-4" />
                <Skeleton className="h-8 w-24 mb-2" />
                <Skeleton className="h-3 w-32" />
              </div>
            ))}
          </div>
          <Skeleton className="h-10 w-full mb-16" />
        </div>
      </section>
      <Footer />
    </div>
  );
}

function ErrorState({ error, onRetry }: { error: string; onRetry: () => void }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <section className="pt-32 pb-16 px-4 sm:px-6 lg:px-12">
        <div className="max-w-5xl mx-auto text-center">
          <XCircle size={48} strokeWidth={1} className="text-red-500 mx-auto mb-6" />
          <h1 className="text-2xl font-light mb-4">Unable to Load Status</h1>
          <p className="text-sm text-muted-foreground mb-8">{error}</p>
          <button
            onClick={onRetry}
            className="inline-flex items-center gap-2 px-4 py-2 border border-border font-mono text-sm hover:bg-secondary transition-colors"
          >
            <RefreshCw size={14} />
            Retry
          </button>
        </div>
      </section>
      <Footer />
    </div>
  );
}

function UptimeChart({ history }: { history: APIStatusUptimeDay[] }) {
  // Pad to 30 days if needed
  const days = history.length > 0 ? history : [];

  return (
    <div className="mb-16">
      <div className="flex items-center justify-between mb-4">
        <h2 className="font-mono text-xs tracking-[0.2em] text-muted-foreground">
          30-DAY UPTIME HISTORY
        </h2>
        <div className="flex items-center gap-4 text-xs font-mono text-muted-foreground">
          <span className="flex items-center gap-1.5">
            <span className="w-2 h-2 bg-emerald-500" /> Operational
          </span>
          <span className="flex items-center gap-1.5">
            <span className="w-2 h-2 bg-amber-500" /> Degraded
          </span>
          <span className="flex items-center gap-1.5">
            <span className="w-2 h-2 bg-red-500" /> Outage
          </span>
        </div>
      </div>
      <div className="flex gap-0.5 sm:gap-1 overflow-x-auto pb-2">
        {days.length > 0 ? (
          days.map((day, index) => (
            <div key={index} className="flex flex-col items-center gap-2 group">
              <div
                className={`w-2 sm:w-3 h-8 sm:h-10 transition-all ${
                  day.status === "operational"
                    ? "bg-emerald-500 hover:bg-emerald-400"
                    : day.status === "degraded"
                      ? "bg-amber-500 hover:bg-amber-400"
                      : "bg-red-500 hover:bg-red-400"
                }`}
                title={`${day.date}: ${day.status}`}
              />
              {index === 0 && (
                <span className="text-[8px] sm:text-[10px] font-mono text-muted-foreground">Today</span>
              )}
              {index === days.length - 1 && days.length > 1 && (
                <span className="text-[8px] sm:text-[10px] font-mono text-muted-foreground">
                  {days.length}d
                </span>
              )}
            </div>
          ))
        ) : (
          <div className="w-full text-center py-4">
            <p className="text-xs text-muted-foreground font-mono">
              Uptime history will appear as health checks accumulate
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

export default function StatusPage() {
  const { data, loading, error, refetch } = useStatus();

  if (loading && !data) {
    return <LoadingSkeleton />;
  }

  if (error && !data) {
    return <ErrorState error={error} onRetry={refetch} />;
  }

  const allOperational = data?.overall_status === "operational";

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />

      {/* Hero Section */}
      <section className="pt-32 pb-16 px-4 sm:px-6 lg:px-12">
        <div className="max-w-5xl mx-auto">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-6 mb-12">
            <div>
              <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
                SYSTEM STATUS
              </p>
              <h1 className="text-3xl sm:text-4xl lg:text-5xl font-light tracking-tight">
                Solvr Status
              </h1>
            </div>
            <div className="flex items-center gap-3">
              {allOperational ? (
                <div className="flex items-center gap-3 px-4 py-3 bg-emerald-50 border border-emerald-200">
                  <span className="relative flex h-3 w-3">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                    <span className="relative inline-flex rounded-full h-3 w-3 bg-emerald-500" />
                  </span>
                  <span className="font-mono text-sm text-emerald-700">All Systems Operational</span>
                </div>
              ) : (
                <div className="flex items-center gap-3 px-4 py-3 bg-amber-50 border border-amber-200">
                  <span className="relative flex h-3 w-3">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
                    <span className="relative inline-flex rounded-full h-3 w-3 bg-amber-500" />
                  </span>
                  <span className="font-mono text-sm text-amber-700">Partial Service Disruption</span>
                </div>
              )}
            </div>
          </div>

          {/* Overall Stats */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-px bg-border border border-border mb-16">
            <div className="bg-background p-4 sm:p-6">
              <Activity size={18} strokeWidth={1.5} className="text-muted-foreground mb-4" />
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">
                {data?.summary.uptime_30d != null ? `${data.summary.uptime_30d.toFixed(2)}%` : "—"}
              </p>
              <p className="text-xs text-muted-foreground font-mono">Overall Uptime (30d)</p>
            </div>
            <div className="bg-background p-4 sm:p-6">
              <Clock size={18} strokeWidth={1.5} className="text-muted-foreground mb-4" />
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">
                {data?.summary.avg_response_time_ms != null
                  ? `${Math.round(data.summary.avg_response_time_ms)}ms`
                  : "—"}
              </p>
              <p className="text-xs text-muted-foreground font-mono">Avg Response Time</p>
            </div>
            <div className="bg-background p-4 sm:p-6">
              <Server size={18} strokeWidth={1.5} className="text-muted-foreground mb-4" />
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">
                {data?.summary.service_count ?? 0}
              </p>
              <p className="text-xs text-muted-foreground font-mono">Active Services</p>
            </div>
            <div className="bg-background p-4 sm:p-6">
              <RefreshCw size={18} strokeWidth={1.5} className="text-muted-foreground mb-4" />
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">
                {data?.summary.last_checked
                  ? formatRelativeTime(data.summary.last_checked)
                  : "—"}
              </p>
              <p className="text-xs text-muted-foreground font-mono">Last Checked</p>
            </div>
          </div>

          {/* 30-Day Uptime Chart */}
          <UptimeChart history={data?.uptime_history ?? []} />
        </div>
      </section>

      {/* Services Section */}
      <section className="py-16 px-4 sm:px-6 lg:px-12 bg-secondary/30">
        <div className="max-w-5xl mx-auto">
          <h2 className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-8">
            SERVICE STATUS
          </h2>

          <div className="space-y-8">
            {(data?.services ?? []).map((category) => (
              <div key={category.category} className="bg-background border border-border">
                <div className="px-4 sm:px-6 py-4 border-b border-border flex items-center gap-3">
                  {getCategoryIcon(category.category)}
                  <h3 className="font-mono text-sm">{category.category}</h3>
                  <span className="text-xs text-muted-foreground font-mono ml-auto">
                    {category.items.filter((s) => s.status === "operational").length}/{category.items.length} operational
                  </span>
                </div>
                <div className="px-4 sm:px-6">
                  {category.items.map((service) => (
                    <ServiceCard key={service.name} service={service} />
                  ))}
                </div>
              </div>
            ))}

            {(data?.services ?? []).length === 0 && (
              <div className="border border-border p-12 text-center">
                <Server size={32} strokeWidth={1} className="text-muted-foreground mx-auto mb-4" />
                <p className="font-mono text-sm mb-2">No service data yet</p>
                <p className="text-xs text-muted-foreground">
                  Health checks will appear after the first monitoring cycle
                </p>
              </div>
            )}
          </div>
        </div>
      </section>

      {/* Recent Incidents */}
      <section className="py-16 px-4 sm:px-6 lg:px-12">
        <div className="max-w-5xl mx-auto">
          <h2 className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-8">
            RECENT INCIDENTS
          </h2>

          <div className="space-y-4">
            {(data?.incidents ?? []).map((incident) => (
              <IncidentCard key={incident.id} incident={incident} />
            ))}
          </div>

          {(data?.incidents ?? []).length === 0 && (
            <div className="border border-border p-12 text-center">
              <CheckCircle2 size={32} strokeWidth={1} className="text-emerald-500 mx-auto mb-4" />
              <p className="font-mono text-sm mb-2">No recent incidents</p>
              <p className="text-xs text-muted-foreground">All systems have been operating normally</p>
            </div>
          )}
        </div>
      </section>

      {/* API Status */}
      <section className="py-16 px-4 sm:px-6 lg:px-12">
        <div className="max-w-5xl mx-auto">
          <h2 className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-8">
            PROGRAMMATIC ACCESS
          </h2>

          <div className="border border-border p-6">
            <h3 className="font-mono text-sm mb-3">Status API</h3>
            <p className="text-sm text-muted-foreground mb-4">
              Get real-time status updates via our JSON API.
            </p>
            <code className="block bg-secondary p-3 font-mono text-xs text-muted-foreground overflow-x-auto">
              GET https://api.solvr.dev/v1/status
            </code>
          </div>
        </div>
      </section>

      <Footer />
    </div>
  );
}
