"use client";

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import {
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Activity,
  Database,
  Server,
  Globe,
  Zap,
  Clock,
  ArrowUpRight,
  ChevronDown,
  ChevronUp,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";

type ServiceStatus = "operational" | "degraded" | "outage";

interface Service {
  name: string;
  description: string;
  status: ServiceStatus;
  uptime: string;
  latency?: string;
  lastChecked: string;
}

interface Incident {
  id: string;
  title: string;
  status: "investigating" | "identified" | "monitoring" | "resolved";
  severity: "minor" | "major" | "critical";
  createdAt: string;
  updatedAt: string;
  updates: {
    time: string;
    message: string;
    status: string;
  }[];
}

const services: { category: string; items: Service[] }[] = [
  {
    category: "Core API",
    items: [
      {
        name: "REST API",
        description: "Primary API endpoints for all operations",
        status: "operational",
        uptime: "99.98%",
        latency: "45ms",
        lastChecked: "2 min ago",
      },
      {
        name: "GraphQL API",
        description: "Query and mutation endpoints",
        status: "operational",
        uptime: "99.97%",
        latency: "52ms",
        lastChecked: "2 min ago",
      },
      {
        name: "Webhooks",
        description: "Event delivery system",
        status: "operational",
        uptime: "99.95%",
        latency: "120ms",
        lastChecked: "2 min ago",
      },
      {
        name: "Authentication",
        description: "OAuth, API keys, and session management",
        status: "operational",
        uptime: "99.99%",
        latency: "28ms",
        lastChecked: "2 min ago",
      },
    ],
  },
  {
    category: "Database",
    items: [
      {
        name: "Primary Database",
        description: "Main PostgreSQL cluster",
        status: "operational",
        uptime: "99.99%",
        latency: "8ms",
        lastChecked: "1 min ago",
      },
      {
        name: "Read Replicas",
        description: "Distributed read nodes",
        status: "operational",
        uptime: "99.98%",
        latency: "12ms",
        lastChecked: "1 min ago",
      },
      {
        name: "Search Index",
        description: "Full-text search infrastructure",
        status: "operational",
        uptime: "99.96%",
        latency: "35ms",
        lastChecked: "2 min ago",
      },
      {
        name: "Cache Layer",
        description: "Redis caching infrastructure",
        status: "operational",
        uptime: "99.99%",
        latency: "2ms",
        lastChecked: "1 min ago",
      },
    ],
  },
  {
    category: "MCP Server",
    items: [
      {
        name: "MCP Cloud",
        description: "Managed MCP server instances",
        status: "operational",
        uptime: "99.94%",
        latency: "65ms",
        lastChecked: "2 min ago",
      },
      {
        name: "Tool Registry",
        description: "MCP tool discovery and routing",
        status: "operational",
        uptime: "99.97%",
        latency: "40ms",
        lastChecked: "2 min ago",
      },
      {
        name: "Agent Gateway",
        description: "AI agent connection handling",
        status: "operational",
        uptime: "99.95%",
        latency: "55ms",
        lastChecked: "2 min ago",
      },
    ],
  },
  {
    category: "Infrastructure",
    items: [
      {
        name: "CDN",
        description: "Global content delivery network",
        status: "operational",
        uptime: "99.99%",
        latency: "15ms",
        lastChecked: "1 min ago",
      },
      {
        name: "File Storage",
        description: "Object storage for attachments",
        status: "operational",
        uptime: "99.99%",
        latency: "25ms",
        lastChecked: "2 min ago",
      },
      {
        name: "Email Delivery",
        description: "Transactional email service",
        status: "operational",
        uptime: "99.92%",
        lastChecked: "5 min ago",
      },
    ],
  },
];

const recentIncidents: Incident[] = [
  {
    id: "INC-2026-0127",
    title: "Elevated API latency in EU region",
    status: "resolved",
    severity: "minor",
    createdAt: "Jan 27, 2026 14:32 UTC",
    updatedAt: "Jan 27, 2026 15:18 UTC",
    updates: [
      {
        time: "15:18 UTC",
        message: "Issue has been fully resolved. All systems operating normally.",
        status: "resolved",
      },
      {
        time: "14:58 UTC",
        message: "Fix deployed. Monitoring for stability.",
        status: "monitoring",
      },
      {
        time: "14:45 UTC",
        message: "Root cause identified: misconfigured load balancer in EU-WEST-1.",
        status: "identified",
      },
      {
        time: "14:32 UTC",
        message: "Investigating reports of elevated latency for EU-based API requests.",
        status: "investigating",
      },
    ],
  },
  {
    id: "INC-2026-0115",
    title: "Scheduled maintenance: Database migration",
    status: "resolved",
    severity: "minor",
    createdAt: "Jan 15, 2026 02:00 UTC",
    updatedAt: "Jan 15, 2026 02:45 UTC",
    updates: [
      {
        time: "02:45 UTC",
        message: "Maintenance completed successfully. All services restored.",
        status: "resolved",
      },
      {
        time: "02:00 UTC",
        message: "Beginning scheduled database maintenance.",
        status: "investigating",
      },
    ],
  },
];

const uptimeHistory = [
  { date: "Today", status: "operational" as const },
  { date: "Feb 2", status: "operational" as const },
  { date: "Feb 1", status: "operational" as const },
  { date: "Jan 31", status: "operational" as const },
  { date: "Jan 30", status: "operational" as const },
  { date: "Jan 29", status: "operational" as const },
  { date: "Jan 28", status: "operational" as const },
  { date: "Jan 27", status: "degraded" as const },
  { date: "Jan 26", status: "operational" as const },
  { date: "Jan 25", status: "operational" as const },
  { date: "Jan 24", status: "operational" as const },
  { date: "Jan 23", status: "operational" as const },
  { date: "Jan 22", status: "operational" as const },
  { date: "Jan 21", status: "operational" as const },
  { date: "Jan 20", status: "operational" as const },
  { date: "Jan 19", status: "operational" as const },
  { date: "Jan 18", status: "operational" as const },
  { date: "Jan 17", status: "operational" as const },
  { date: "Jan 16", status: "operational" as const },
  { date: "Jan 15", status: "degraded" as const },
  { date: "Jan 14", status: "operational" as const },
  { date: "Jan 13", status: "operational" as const },
  { date: "Jan 12", status: "operational" as const },
  { date: "Jan 11", status: "operational" as const },
  { date: "Jan 10", status: "operational" as const },
  { date: "Jan 9", status: "operational" as const },
  { date: "Jan 8", status: "operational" as const },
  { date: "Jan 7", status: "operational" as const },
  { date: "Jan 6", status: "operational" as const },
  { date: "Jan 5", status: "operational" as const },
];

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

function IncidentStatusBadge({ status }: { status: Incident["status"] }) {
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

function ServiceCard({ service }: { service: Service }) {
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
        {service.latency && (
          <div className="hidden md:block">
            <span className="text-foreground">{service.latency}</span> avg
          </div>
        )}
        <div className="text-[10px]">{service.lastChecked}</div>
      </div>
    </div>
  );
}

function IncidentCard({ incident }: { incident: Incident }) {
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
            {incident.createdAt} — {incident.updatedAt}
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

export default function StatusPage() {
  const allOperational = services.every((category) =>
    category.items.every((service) => service.status === "operational")
  );

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
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">99.97%</p>
              <p className="text-xs text-muted-foreground font-mono">Overall Uptime (30d)</p>
            </div>
            <div className="bg-background p-4 sm:p-6">
              <Clock size={18} strokeWidth={1.5} className="text-muted-foreground mb-4" />
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">42ms</p>
              <p className="text-xs text-muted-foreground font-mono">Avg Response Time</p>
            </div>
            <div className="bg-background p-4 sm:p-6">
              <Server size={18} strokeWidth={1.5} className="text-muted-foreground mb-4" />
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">15</p>
              <p className="text-xs text-muted-foreground font-mono">Active Services</p>
            </div>
            <div className="bg-background p-4 sm:p-6">
              <Globe size={18} strokeWidth={1.5} className="text-muted-foreground mb-4" />
              <p className="font-mono text-2xl sm:text-3xl font-light mb-1">6</p>
              <p className="text-xs text-muted-foreground font-mono">Global Regions</p>
            </div>
          </div>

          {/* 30-Day Uptime Chart */}
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
              {uptimeHistory.map((day, index) => (
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
                  {index === uptimeHistory.length - 1 && (
                    <span className="text-[8px] sm:text-[10px] font-mono text-muted-foreground">30d</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* Services Section */}
      <section className="py-16 px-4 sm:px-6 lg:px-12 bg-secondary/30">
        <div className="max-w-5xl mx-auto">
          <h2 className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-8">
            SERVICE STATUS
          </h2>

          <div className="space-y-8">
            {services.map((category) => (
              <div key={category.category} className="bg-background border border-border">
                <div className="px-4 sm:px-6 py-4 border-b border-border flex items-center gap-3">
                  {category.category === "Core API" && <Server size={16} strokeWidth={1.5} />}
                  {category.category === "Database" && <Database size={16} strokeWidth={1.5} />}
                  {category.category === "MCP Server" && <Zap size={16} strokeWidth={1.5} />}
                  {category.category === "Infrastructure" && <Globe size={16} strokeWidth={1.5} />}
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
            {recentIncidents.map((incident) => (
              <IncidentCard key={incident.id} incident={incident} />
            ))}
          </div>

          {recentIncidents.length === 0 && (
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

          <div className="grid md:grid-cols-2 gap-6">
            <div className="border border-border p-6">
              <h3 className="font-mono text-sm mb-3">Status API</h3>
              <p className="text-sm text-muted-foreground mb-4">
                Get real-time status updates via our JSON API.
              </p>
              <code className="block bg-secondary p-3 font-mono text-xs text-muted-foreground overflow-x-auto">
                GET https://api.solvr.dev/v1/status
              </code>
            </div>
            <div className="border border-border p-6">
              <h3 className="font-mono text-sm mb-3">Webhook Notifications</h3>
              <p className="text-sm text-muted-foreground mb-4">
                Receive instant notifications for status changes.
              </p>
              <Link
                href="/api-docs"
                className="font-mono text-xs flex items-center gap-1 hover:text-muted-foreground transition-colors"
              >
                Configure webhooks
                <ArrowUpRight size={12} />
              </Link>
            </div>
          </div>
        </div>
      </section>

      <Footer />
    </div>
  );
}
