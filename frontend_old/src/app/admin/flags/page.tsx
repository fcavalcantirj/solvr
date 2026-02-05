'use client';

/**
 * Admin Flags Page
 * Per SPEC.md Part 16.3 and PRD lines 520-522
 *
 * Features:
 * - Requires admin role (admin or super_admin)
 * - List pending flags with content preview
 * - Action buttons: dismiss, warn, hide, delete
 * - Filtering by status and target type
 * - Pagination
 */

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/lib/api';

// Types for flags data
interface ContentPreview {
  title?: string;
  text?: string;
  snippet?: string;
  type: string;
}

interface Reporter {
  id: string;
  display_name: string;
  type: string;
}

interface Flag {
  id: string;
  target_type: string;
  target_id: string;
  reporter_type: string;
  reporter_id: string;
  reason: string;
  details?: string;
  status: string;
  created_at: string;
  content_preview?: ContentPreview;
  reporter?: Reporter;
}

interface FlagsResponse {
  data: Flag[];
  total: number;
  page: number;
}

// Helper to check if user has admin role
function isAdminRole(role?: string): boolean {
  return role === 'admin' || role === 'super_admin';
}

// Loading skeleton
function FlagSkeleton() {
  return (
    <div className="bg-white rounded-lg shadow p-4 animate-pulse">
      <div className="h-4 bg-gray-200 rounded w-1/4 mb-2"></div>
      <div className="h-6 bg-gray-200 rounded w-3/4 mb-2"></div>
      <div className="h-4 bg-gray-200 rounded w-1/2"></div>
    </div>
  );
}

// Reason badge colors
const reasonColors: Record<string, string> = {
  spam: 'bg-red-100 text-red-800',
  offensive: 'bg-orange-100 text-orange-800',
  duplicate: 'bg-blue-100 text-blue-800',
  incorrect: 'bg-yellow-100 text-yellow-800',
  low_quality: 'bg-gray-100 text-gray-800',
  other: 'bg-purple-100 text-purple-800',
};

// Target type badge colors
const targetTypeColors: Record<string, string> = {
  post: 'bg-indigo-100 text-indigo-800',
  comment: 'bg-teal-100 text-teal-800',
  answer: 'bg-green-100 text-green-800',
  approach: 'bg-cyan-100 text-cyan-800',
  response: 'bg-pink-100 text-pink-800',
};

// Flag card component
interface FlagCardProps {
  flag: Flag;
  onDismiss: () => void;
  onWarn: () => void;
  onHide: () => void;
  onDelete: () => void;
  isLoading: boolean;
}

function FlagCard({ flag, onDismiss, onWarn, onHide, onDelete, isLoading }: FlagCardProps) {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  return (
    <div className="bg-white rounded-lg shadow p-4 mb-4">
      {/* Header with badges */}
      <div className="flex items-center gap-2 mb-3">
        <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${targetTypeColors[flag.target_type] || 'bg-gray-100'}`}>
          {flag.target_type}
        </span>
        <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${reasonColors[flag.reason] || 'bg-gray-100'}`}>
          {flag.reason}
        </span>
        <span className="text-xs text-gray-400 ml-auto">
          {new Date(flag.created_at).toLocaleDateString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
          })}
        </span>
      </div>

      {/* Content preview */}
      {flag.content_preview && (
        <div className="bg-gray-50 rounded p-3 mb-3">
          {flag.content_preview.title && (
            <h4 className="font-medium text-gray-900 mb-1">{flag.content_preview.title}</h4>
          )}
          {(flag.content_preview.snippet || flag.content_preview.text) && (
            <p className="text-sm text-gray-600">
              {flag.content_preview.snippet || flag.content_preview.text}
            </p>
          )}
        </div>
      )}

      {/* Flag details */}
      {flag.details && (
        <p className="text-sm text-gray-700 mb-3">
          <span className="font-medium">Reason: </span>
          {flag.details}
        </p>
      )}

      {/* Reporter info */}
      {flag.reporter && (
        <p className="text-xs text-gray-500 mb-3">
          Reported by {flag.reporter.display_name} ({flag.reporter.type})
        </p>
      )}

      {/* View content link */}
      <div className="flex items-center justify-between">
        <Link
          href={`/${flag.target_type}s/${flag.target_id}`}
          className="text-sm text-blue-600 hover:text-blue-800"
        >
          View content →
        </Link>

        {/* Action buttons */}
        <div className="flex gap-2">
          <button
            onClick={onDismiss}
            disabled={isLoading}
            className="px-3 py-1 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200 disabled:opacity-50"
          >
            Dismiss
          </button>
          <button
            onClick={onWarn}
            disabled={isLoading}
            className="px-3 py-1 text-sm bg-yellow-100 text-yellow-700 rounded hover:bg-yellow-200 disabled:opacity-50"
          >
            Warn
          </button>
          <button
            onClick={onHide}
            disabled={isLoading}
            className="px-3 py-1 text-sm bg-orange-100 text-orange-700 rounded hover:bg-orange-200 disabled:opacity-50"
          >
            Hide
          </button>
          {showDeleteConfirm ? (
            <div className="flex gap-1">
              <span className="text-sm text-red-600 px-2 py-1">Are you sure?</span>
              <button
                onClick={() => {
                  onDelete();
                  setShowDeleteConfirm(false);
                }}
                disabled={isLoading}
                className="px-3 py-1 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50"
              >
                Yes
              </button>
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="px-3 py-1 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
              >
                No
              </button>
            </div>
          ) : (
            <button
              onClick={() => setShowDeleteConfirm(true)}
              disabled={isLoading}
              className="px-3 py-1 text-sm bg-red-100 text-red-700 rounded hover:bg-red-200 disabled:opacity-50"
            >
              Delete
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

export default function AdminFlagsPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [flags, setFlags] = useState<Flag[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [processingFlagId, setProcessingFlagId] = useState<string | null>(null);

  // Filters
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string>('');

  const perPage = 20;

  // Fetch flags
  const fetchFlags = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const params = new URLSearchParams();
      params.set('page', page.toString());
      params.set('per_page', perPage.toString());
      if (statusFilter) params.set('status', statusFilter);
      if (typeFilter) params.set('target_type', typeFilter);

      const response = await api.get<FlagsResponse>(`/v1/admin/flags?${params.toString()}`);

      setFlags(response.data || []);
      setTotal(response.total || 0);
    } catch (err) {
      setError('Failed to load flags');
      console.error('Flags fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, [page, statusFilter, typeFilter]);

  // Check auth and fetch flags
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

    fetchFlags();
  }, [user, authLoading, router, fetchFlags]);

  // Action handlers
  const handleDismiss = async (flagId: string) => {
    try {
      setProcessingFlagId(flagId);
      setActionError(null);
      await api.post(`/v1/admin/flags/${flagId}/dismiss`, {});
      setFlags(flags.filter(f => f.id !== flagId));
    } catch (err) {
      setActionError('Action failed. Please try again.');
      console.error('Dismiss error:', err);
    } finally {
      setProcessingFlagId(null);
    }
  };

  const handleWarn = async (flagId: string) => {
    try {
      setProcessingFlagId(flagId);
      setActionError(null);
      await api.post(`/v1/admin/flags/${flagId}/action`, { action: 'warn' });
      setFlags(flags.filter(f => f.id !== flagId));
    } catch (err) {
      setActionError('Action failed. Please try again.');
      console.error('Warn error:', err);
    } finally {
      setProcessingFlagId(null);
    }
  };

  const handleHide = async (flagId: string) => {
    try {
      setProcessingFlagId(flagId);
      setActionError(null);
      await api.post(`/v1/admin/flags/${flagId}/action`, { action: 'hide' });
      setFlags(flags.filter(f => f.id !== flagId));
    } catch (err) {
      setActionError('Action failed. Please try again.');
      console.error('Hide error:', err);
    } finally {
      setProcessingFlagId(null);
    }
  };

  const handleDelete = async (flagId: string) => {
    try {
      setProcessingFlagId(flagId);
      setActionError(null);
      await api.post(`/v1/admin/flags/${flagId}/action`, { action: 'delete' });
      setFlags(flags.filter(f => f.id !== flagId));
    } catch (err) {
      setActionError('Action failed. Please try again.');
      console.error('Delete error:', err);
    } finally {
      setProcessingFlagId(null);
    }
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
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <Link
            href="/admin"
            className="text-sm text-blue-600 hover:text-blue-800 mb-2 inline-block"
          >
            ← Back to Admin
          </Link>
          <h1 className="text-3xl font-bold text-gray-900">Flags</h1>
          <p className="mt-1 text-sm text-gray-500">
            Review and moderate flagged content
          </p>
        </div>

        {/* Filters */}
        <div className="bg-white rounded-lg shadow p-4 mb-6 flex gap-4 flex-wrap">
          <div>
            <label htmlFor="status-filter" className="block text-sm font-medium text-gray-700 mb-1">
              Status
            </label>
            <select
              id="status-filter"
              aria-label="Status"
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value);
                setPage(1);
              }}
              className="block w-40 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"
            >
              <option value="">All</option>
              <option value="pending">Pending</option>
              <option value="reviewed">Reviewed</option>
              <option value="dismissed">Dismissed</option>
              <option value="actioned">Actioned</option>
            </select>
          </div>

          <div>
            <label htmlFor="type-filter" className="block text-sm font-medium text-gray-700 mb-1">
              Type
            </label>
            <select
              id="type-filter"
              aria-label="Type"
              value={typeFilter}
              onChange={(e) => {
                setTypeFilter(e.target.value);
                setPage(1);
              }}
              className="block w-40 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"
            >
              <option value="">All</option>
              <option value="post">Post</option>
              <option value="comment">Comment</option>
              <option value="answer">Answer</option>
              <option value="approach">Approach</option>
              <option value="response">Response</option>
            </select>
          </div>
        </div>

        {/* Action error message */}
        {actionError && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-4">
            <p className="text-red-700">{actionError}</p>
          </div>
        )}

        {/* Error State */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-4 flex items-center justify-between">
            <p className="text-red-700">{error}</p>
            <button
              onClick={fetchFlags}
              className="px-4 py-2 bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors"
            >
              Retry
            </button>
          </div>
        )}

        {/* Loading State */}
        {loading ? (
          <div role="status" aria-label="Loading flags">
            <FlagSkeleton />
            <FlagSkeleton />
            <FlagSkeleton />
          </div>
        ) : flags.length === 0 ? (
          /* Empty State */
          <div className="bg-white rounded-lg shadow p-8 text-center">
            <p className="text-gray-500">No flags found</p>
          </div>
        ) : (
          /* Flags List */
          <div>
            {flags.map((flag) => (
              <FlagCard
                key={flag.id}
                flag={flag}
                onDismiss={() => handleDismiss(flag.id)}
                onWarn={() => handleWarn(flag.id)}
                onHide={() => handleHide(flag.id)}
                onDelete={() => handleDelete(flag.id)}
                isLoading={processingFlagId === flag.id}
              />
            ))}
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <nav aria-label="Pagination" className="flex items-center justify-center gap-4 mt-6">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-4 py-2 bg-white border rounded-md disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Previous
            </button>
            <span className="text-sm text-gray-700">
              Page {page} of {totalPages}
            </span>
            <button
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
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
