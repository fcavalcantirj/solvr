'use client';

/**
 * Admin Dashboard Page
 * Per SPEC.md Part 16.3 and PRD lines 517-519
 *
 * Features:
 * - Requires admin role (admin or super_admin)
 * - Overview stats from /v1/admin/stats
 * - Quick links to sub-pages (flags, users, agents, audit)
 * - Recent flags preview
 */

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/lib/api';

// Types for admin data
interface AdminStats {
  users_count: number;
  agents_count: number;
  posts_count: number;
  flags_count: number;
  rate_limit_hits: number;
  active_users_24h: number;
}

interface Flag {
  id: string;
  target_type: string;
  target_id: string;
  reporter_type: string;
  reporter_id: string;
  reason: string;
  status: string;
  created_at: string;
}

interface FlagsResponse {
  data: Flag[];
  total: number;
}

// Helper to check if user has admin role
function isAdminRole(role?: string): boolean {
  return role === 'admin' || role === 'super_admin';
}

// Format large numbers with commas
function formatNumber(num: number): string {
  return num.toLocaleString();
}

// Stat card skeleton for loading state
function StatSkeleton() {
  return (
    <div
      data-testid="stat-skeleton"
      className="bg-white rounded-lg shadow p-6 animate-pulse"
    >
      <div className="h-4 bg-gray-200 rounded w-20 mb-2"></div>
      <div className="h-8 bg-gray-200 rounded w-16"></div>
    </div>
  );
}

// Stat card component
interface StatCardProps {
  label: string;
  value: number;
  icon?: React.ReactNode;
  href?: string;
}

function StatCard({ label, value, icon, href }: StatCardProps) {
  const content = (
    <div className="bg-white rounded-lg shadow p-6 hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-500">{label}</p>
          <p className="text-2xl font-bold text-gray-900">{formatNumber(value)}</p>
        </div>
        {icon && (
          <div className="text-gray-400 text-2xl">{icon}</div>
        )}
      </div>
    </div>
  );

  if (href) {
    return <Link href={href}>{content}</Link>;
  }
  return content;
}

// Quick link card component
interface QuickLinkProps {
  href: string;
  title: string;
  description: string;
  icon: React.ReactNode;
}

function QuickLink({ href, title, description, icon }: QuickLinkProps) {
  return (
    <Link
      href={href}
      className="bg-white rounded-lg shadow p-6 hover:shadow-md transition-shadow flex items-start gap-4"
    >
      <div className="text-blue-500 text-2xl flex-shrink-0">{icon}</div>
      <div>
        <h3 className="font-semibold text-gray-900">{title}</h3>
        <p className="text-sm text-gray-500">{description}</p>
      </div>
    </Link>
  );
}

// Flag item component
interface FlagItemProps {
  flag: Flag;
}

function FlagItem({ flag }: FlagItemProps) {
  return (
    <div className="flex items-center justify-between py-3 border-b border-gray-100 last:border-0">
      <div className="flex items-center gap-3">
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
          {flag.reason}
        </span>
        <span className="text-sm text-gray-500">on {flag.target_type}</span>
      </div>
      <span className="text-xs text-gray-400">
        {new Date(flag.created_at).toLocaleDateString()}
      </span>
    </div>
  );
}

// Icons (inline SVGs to avoid dependencies)
const Icons = {
  users: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  ),
  agents: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  ),
  posts: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  ),
  flags: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 21v-4m0 0V5a2 2 0 012-2h6.5l1 1H21l-3 6 3 6h-8.5l-1-1H5a2 2 0 00-2 2zm9-13.5V9" />
    </svg>
  ),
  active: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  audit: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
    </svg>
  ),
};

export default function AdminPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [stats, setStats] = useState<AdminStats | null>(null);
  const [recentFlags, setRecentFlags] = useState<Flag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch admin data
  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const [statsData, flagsData] = await Promise.all([
        api.get<AdminStats>('/v1/admin/stats'),
        api.get<FlagsResponse>('/v1/admin/flags?status=pending&per_page=5'),
      ]);

      setStats(statsData);
      setRecentFlags(flagsData.data || []);
    } catch (err) {
      setError('Failed to load admin data');
      console.error('Admin data fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  // Check auth and redirect non-admins
  useEffect(() => {
    if (authLoading) return;

    if (!user) {
      router.replace('/login');
      return;
    }

    if (!isAdminRole(user.role)) {
      router.replace('/');
      return;
    }

    fetchData();
  }, [user, authLoading, router, fetchData]);

  // Loading state while checking auth
  if (authLoading) {
    return (
      <div
        role="status"
        aria-label="Loading admin panel"
        className="min-h-screen flex items-center justify-center"
      >
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
      </div>
    );
  }

  // Don't render content if not authorized (redirect happens in useEffect)
  if (!user || !isAdminRole(user.role)) {
    return null;
  }

  return (
    <main className="min-h-screen bg-gray-50 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Admin Dashboard</h1>
          <p className="mt-1 text-sm text-gray-500">
            System overview and management tools
          </p>
        </div>

        {/* Error State */}
        {error && (
          <div className="mb-8 bg-red-50 border border-red-200 rounded-lg p-4 flex items-center justify-between">
            <p className="text-red-700">{error}</p>
            <button
              onClick={fetchData}
              className="px-4 py-2 bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors"
            >
              Retry
            </button>
          </div>
        )}

        {/* Stats Grid */}
        <section aria-label="Statistics" role="region" className="mb-8">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Overview</h2>
          <div data-testid="stats-grid" className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
            {loading ? (
              <>
                <StatSkeleton />
                <StatSkeleton />
                <StatSkeleton />
                <StatSkeleton />
                <StatSkeleton />
              </>
            ) : stats ? (
              <>
                <StatCard
                  label="Users"
                  value={stats.users_count}
                  icon={Icons.users}
                  href="/admin/users"
                />
                <StatCard
                  label="Agents"
                  value={stats.agents_count}
                  icon={Icons.agents}
                  href="/admin/agents"
                />
                <StatCard
                  label="Posts"
                  value={stats.posts_count}
                  icon={Icons.posts}
                />
                <StatCard
                  label="Pending Flags"
                  value={stats.flags_count}
                  icon={Icons.flags}
                  href="/admin/flags"
                />
                <StatCard
                  label="Active 24h"
                  value={stats.active_users_24h}
                  icon={Icons.active}
                />
              </>
            ) : null}
          </div>
        </section>

        {/* Quick Links */}
        <nav aria-label="Admin navigation" className="mb-8">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Quick Actions</h2>
          <div data-testid="quick-links" className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <QuickLink
              href="/admin/flags"
              title="Flags"
              description="Review and moderate flagged content"
              icon={Icons.flags}
            />
            <QuickLink
              href="/admin/users"
              title="Users"
              description="Manage user accounts and permissions"
              icon={Icons.users}
            />
            <QuickLink
              href="/admin/agents"
              title="Manage Agents"
              description="View and manage AI agents"
              icon={Icons.agents}
            />
            <QuickLink
              href="/admin/audit"
              title="Audit Log"
              description="View admin action history"
              icon={Icons.audit}
            />
          </div>
        </nav>

        {/* Recent Flags */}
        <section className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-gray-900">Recent Flags</h2>
            <Link
              href="/admin/flags"
              className="text-sm text-blue-600 hover:text-blue-800"
            >
              View all flags →
            </Link>
          </div>

          {loading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-12 bg-gray-100 rounded animate-pulse"></div>
              ))}
            </div>
          ) : recentFlags.length === 0 ? (
            <p className="text-gray-500 text-center py-8">No pending flags</p>
          ) : (
            <div>
              {recentFlags.map((flag) => (
                <FlagItem key={flag.id} flag={flag} />
              ))}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
