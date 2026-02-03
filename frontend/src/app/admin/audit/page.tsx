'use client';

/**
 * Admin Audit Log Page
 * Per SPEC.md Part 16.3 and PRD line 526
 *
 * Features:
 * - Requires admin role (admin or super_admin)
 * - List audit log entries with filters
 * - Action filter dropdown
 * - Date range filter
 * - Expandable entry details
 * - Pagination
 */

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/lib/api';

// Types for audit data
interface AuditEntry {
  id: string;
  admin_id: string;
  admin_name?: string;
  action: string;
  target_type: string;
  target_id?: string;
  details?: Record<string, unknown>;
  ip_address?: string;
  created_at: string;
}

interface AuditResponse {
  data: AuditEntry[];
  total: number;
  page: number;
}

// Helper to check if user has admin role
function isAdminRole(role?: string): boolean {
  return role === 'admin' || role === 'super_admin';
}

// Format time for display
function formatDateTime(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// Action badge colors
const actionColors: Record<string, string> = {
  ban_user: 'bg-red-100 text-red-800',
  suspend_user: 'bg-orange-100 text-orange-800',
  warn_user: 'bg-yellow-100 text-yellow-800',
  dismiss_flag: 'bg-blue-100 text-blue-800',
  action_flag: 'bg-purple-100 text-purple-800',
  delete: 'bg-red-100 text-red-800',
  hard_delete_post: 'bg-red-100 text-red-800',
  restore_post: 'bg-green-100 text-green-800',
  revoke_agent_key: 'bg-orange-100 text-orange-800',
  suspend_agent: 'bg-orange-100 text-orange-800',
};

// Target type badge colors
const targetTypeColors: Record<string, string> = {
  user: 'bg-indigo-100 text-indigo-800',
  post: 'bg-teal-100 text-teal-800',
  flag: 'bg-cyan-100 text-cyan-800',
  agent: 'bg-pink-100 text-pink-800',
};

// Loading skeleton
function AuditSkeleton() {
  return (
    <tr className="animate-pulse">
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-32"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-24"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-20"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-16"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-8"></div>
      </td>
    </tr>
  );
}

// Audit entry row component
interface AuditRowProps {
  entry: AuditEntry;
  expanded: boolean;
  onToggle: () => void;
}

function AuditRow({ entry, expanded, onToggle }: AuditRowProps) {
  return (
    <>
      <tr className="border-b border-gray-100 hover:bg-gray-50">
        <td className="px-6 py-4 text-sm text-gray-600">
          {formatDateTime(entry.created_at)}
        </td>
        <td className="px-6 py-4 text-sm text-gray-900 font-medium">
          {entry.admin_name || 'Unknown'}
        </td>
        <td className="px-6 py-4">
          <span
            className={`px-2 py-0.5 rounded-full text-xs font-medium ${
              actionColors[entry.action] || 'bg-gray-100 text-gray-800'
            }`}
          >
            {entry.action}
          </span>
        </td>
        <td className="px-6 py-4">
          <span
            className={`px-2 py-0.5 rounded-full text-xs font-medium ${
              targetTypeColors[entry.target_type] || 'bg-gray-100 text-gray-800'
            }`}
          >
            {entry.target_type}
          </span>
        </td>
        <td className="px-6 py-4">
          <button
            onClick={onToggle}
            aria-label="Expand details"
            className="text-blue-600 hover:text-blue-800 text-sm"
          >
            {expanded ? '▼' : '▶'}
          </button>
        </td>
      </tr>
      {expanded && (
        <tr className="bg-gray-50">
          <td colSpan={5} className="px-6 py-4">
            <div className="text-sm space-y-2">
              {entry.target_id && (
                <p>
                  <span className="text-gray-500">Target ID:</span>{' '}
                  <span className="font-mono text-gray-700">{entry.target_id}</span>
                </p>
              )}
              {entry.ip_address && (
                <p>
                  <span className="text-gray-500">IP Address:</span>{' '}
                  <span className="font-mono text-gray-700">{entry.ip_address}</span>
                </p>
              )}
              {entry.details && Object.keys(entry.details).length > 0 && (
                <div>
                  <span className="text-gray-500">Details:</span>
                  <pre className="mt-1 p-2 bg-gray-100 rounded text-xs overflow-x-auto">
                    {JSON.stringify(entry.details, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

// Available actions for filter
const actionOptions = [
  { value: '', label: 'All Actions' },
  { value: 'ban_user', label: 'Ban User' },
  { value: 'suspend_user', label: 'Suspend User' },
  { value: 'warn_user', label: 'Warn User' },
  { value: 'dismiss_flag', label: 'Dismiss Flag' },
  { value: 'action_flag', label: 'Action Flag' },
  { value: 'hard_delete_post', label: 'Delete Post' },
  { value: 'restore_post', label: 'Restore Post' },
  { value: 'revoke_agent_key', label: 'Revoke Agent Key' },
  { value: 'suspend_agent', label: 'Suspend Agent' },
];

export default function AdminAuditPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  // Filters
  const [actionFilter, setActionFilter] = useState('');
  const [fromDate, setFromDate] = useState('');
  const [toDate, setToDate] = useState('');

  const perPage = 20;

  // Fetch audit entries
  const fetchAudit = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const params = new URLSearchParams();
      params.set('page', page.toString());
      params.set('per_page', perPage.toString());
      if (actionFilter) params.set('action', actionFilter);
      if (fromDate) params.set('from_date', fromDate);
      if (toDate) params.set('to_date', toDate);

      const response = await api.get<AuditResponse>(`/v1/admin/audit?${params.toString()}`);

      setEntries(response.data || []);
      setTotal(response.total || 0);
    } catch (err) {
      setError('Failed to load audit log');
      console.error('Audit fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, [page, actionFilter, fromDate, toDate]);

  // Check auth and fetch audit
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

    fetchAudit();
  }, [user, authLoading, router, fetchAudit]);

  // Handle action filter change
  const handleActionChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setActionFilter(e.target.value);
    setPage(1);
  };

  // Handle from date change
  const handleFromDateChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFromDate(e.target.value);
    setPage(1);
  };

  // Handle to date change
  const handleToDateChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setToDate(e.target.value);
    setPage(1);
  };

  // Toggle entry expansion
  const toggleExpand = (id: string) => {
    setExpandedId(expandedId === id ? null : id);
  };

  // Loading state while checking auth
  if (authLoading) {
    return (
      <div
        role="status"
        aria-label="Loading"
        className="min-h-screen flex items-center justify-center"
      >
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
      </div>
    );
  }

  // Don't render content if not authorized
  if (!user || !isAdminRole(user.role)) {
    return null;
  }

  const totalPages = Math.ceil(total / perPage);

  return (
    <main className="min-h-screen bg-gray-50 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-6xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <Link
            href="/admin"
            className="text-sm text-blue-600 hover:text-blue-800 mb-2 inline-block"
          >
            &larr; Back to Admin
          </Link>
          <h1 className="text-3xl font-bold text-gray-900">Audit Log</h1>
          <p className="mt-1 text-sm text-gray-500">
            View admin action history
          </p>
        </div>

        {/* Filters */}
        <div className="bg-white rounded-lg shadow p-4 mb-6 flex gap-4 flex-wrap items-end">
          <div>
            <label htmlFor="action-filter" className="block text-sm font-medium text-gray-700 mb-1">
              Action
            </label>
            <select
              id="action-filter"
              aria-label="Action"
              value={actionFilter}
              onChange={handleActionChange}
              className="block w-48 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm py-2 px-3 border"
            >
              {actionOptions.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="from-date" className="block text-sm font-medium text-gray-700 mb-1">
              From Date
            </label>
            <input
              id="from-date"
              type="date"
              value={fromDate}
              onChange={handleFromDateChange}
              className="block w-40 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm py-2 px-3 border"
            />
          </div>

          <div>
            <label htmlFor="to-date" className="block text-sm font-medium text-gray-700 mb-1">
              To Date
            </label>
            <input
              id="to-date"
              type="date"
              value={toDate}
              onChange={handleToDateChange}
              className="block w-40 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm py-2 px-3 border"
            />
          </div>
        </div>

        {/* Error State */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-4 flex items-center justify-between">
            <p className="text-red-700">{error}</p>
            <button
              onClick={fetchAudit}
              className="px-4 py-2 bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors"
            >
              Retry
            </button>
          </div>
        )}

        {/* Audit Table */}
        <div className="bg-white rounded-lg shadow overflow-hidden">
          <table role="table" className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Timestamp
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Admin
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Action
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Target
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Details
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {loading ? (
                <tr role="status" aria-label="Loading audit entries">
                  <td colSpan={5}>
                    <div className="p-8">
                      <AuditSkeleton />
                      <AuditSkeleton />
                      <AuditSkeleton />
                    </div>
                  </td>
                </tr>
              ) : entries.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-12 text-center text-gray-500">
                    No audit entries found
                  </td>
                </tr>
              ) : (
                entries.map((entry) => (
                  <AuditRow
                    key={entry.id}
                    entry={entry}
                    expanded={expandedId === entry.id}
                    onToggle={() => toggleExpand(entry.id)}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <nav aria-label="Pagination" className="flex items-center justify-center gap-4 mt-6">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-4 py-2 bg-white border rounded-md disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Previous
            </button>
            <span className="text-sm text-gray-700">
              Page {page} of {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="px-4 py-2 bg-white border rounded-md disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Next
            </button>
          </nav>
        )}
      </div>
    </main>
  );
}
