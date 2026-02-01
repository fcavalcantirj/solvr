'use client';

/**
 * Admin Users Page
 * Per SPEC.md Part 16.3 and PRD line 523
 *
 * Features:
 * - Requires admin role (admin or super_admin)
 * - Paginated user list with search
 * - Status filtering (active, suspended, banned)
 * - Click through to user detail
 */

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/lib/api';

// Types for user data
interface User {
  id: string;
  username: string;
  display_name: string;
  email: string;
  avatar_url?: string;
  auth_provider: string;
  role: string;
  status: string;
  created_at: string;
  updated_at: string;
}

interface UsersResponse {
  data: User[];
  total: number;
  page: number;
}

// Helper to check if user has admin role
function isAdminRole(role?: string): boolean {
  return role === 'admin' || role === 'super_admin';
}

// Status badge colors
const statusColors: Record<string, string> = {
  active: 'bg-green-100 text-green-800',
  suspended: 'bg-yellow-100 text-yellow-800',
  banned: 'bg-red-100 text-red-800',
};

// Loading skeleton for user row
function UserSkeleton() {
  return (
    <tr data-testid="user-skeleton" className="animate-pulse">
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-24"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-32"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-40"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-16"></div>
      </td>
      <td className="px-6 py-4">
        <div className="h-4 bg-gray-200 rounded w-12"></div>
      </td>
    </tr>
  );
}

// User row component
interface UserRowProps {
  user: User;
}

function UserRow({ user }: UserRowProps) {
  return (
    <tr className="border-b border-gray-100 hover:bg-gray-50">
      <td className="px-6 py-4">
        <div className="flex items-center gap-3">
          {user.avatar_url ? (
            <img
              src={user.avatar_url}
              alt={user.display_name}
              className="w-8 h-8 rounded-full"
            />
          ) : (
            <div className="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center">
              <span className="text-gray-500 text-xs">
                {user.display_name.charAt(0).toUpperCase()}
              </span>
            </div>
          )}
          <div>
            <p className="font-medium text-gray-900">{user.username}</p>
            <p className="text-sm text-gray-500">{user.display_name}</p>
          </div>
        </div>
      </td>
      <td className="px-6 py-4 text-sm text-gray-600">{user.email}</td>
      <td className="px-6 py-4">
        <span className="text-xs text-gray-500 capitalize">{user.auth_provider}</span>
      </td>
      <td className="px-6 py-4">
        <span
          className={`px-2 py-0.5 rounded-full text-xs font-medium capitalize ${
            statusColors[user.status] || 'bg-gray-100 text-gray-800'
          }`}
        >
          {user.status}
        </span>
      </td>
      <td className="px-6 py-4">
        <Link
          href={`/admin/users/${user.id}`}
          className="text-sm text-blue-600 hover:text-blue-800"
          aria-label={`View ${user.username}`}
        >
          View
        </Link>
      </td>
    </tr>
  );
}

export default function AdminUsersPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('');

  const perPage = 20;

  // Fetch users
  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const params = new URLSearchParams();
      params.set('page', page.toString());
      params.set('per_page', perPage.toString());
      if (searchQuery) params.set('q', searchQuery);
      if (statusFilter) params.set('status', statusFilter);

      const response = await api.get<UsersResponse>(`/v1/admin/users?${params.toString()}`);

      setUsers(response.data || []);
      setTotal(response.total || 0);
    } catch (err) {
      setError('Failed to load users');
      console.error('Users fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, [page, searchQuery, statusFilter]);

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      if (user && isAdminRole(user.role)) {
        fetchUsers();
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [searchQuery, fetchUsers, user]);

  // Fetch on filter/page change
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

    fetchUsers();
  }, [user, authLoading, router, page, statusFilter, fetchUsers]);

  // Handle search input change
  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchQuery(e.target.value);
    setPage(1); // Reset to first page on new search
  };

  // Handle status filter change
  const handleStatusChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setStatusFilter(e.target.value);
    setPage(1); // Reset to first page on filter change
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
          <h1 className="text-3xl font-bold text-gray-900">Users</h1>
          <p className="mt-1 text-sm text-gray-500">
            Manage user accounts and permissions
          </p>
        </div>

        {/* Filters */}
        <div className="bg-white rounded-lg shadow p-4 mb-6 flex gap-4 flex-wrap items-end">
          <div className="flex-1 min-w-[200px]">
            <label htmlFor="search-users" className="block text-sm font-medium text-gray-700 mb-1">
              Search
            </label>
            <input
              id="search-users"
              type="search"
              role="searchbox"
              placeholder="Search users by name or email..."
              value={searchQuery}
              onChange={handleSearchChange}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm py-2 px-3 border"
            />
          </div>

          <div>
            <label htmlFor="status-filter" className="block text-sm font-medium text-gray-700 mb-1">
              Status
            </label>
            <select
              id="status-filter"
              aria-label="Status"
              value={statusFilter}
              onChange={handleStatusChange}
              className="block w-40 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm py-2 px-3 border"
            >
              <option value="">All</option>
              <option value="active">Active</option>
              <option value="suspended">Suspended</option>
              <option value="banned">Banned</option>
            </select>
          </div>
        </div>

        {/* Error State */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-4 flex items-center justify-between">
            <p className="text-red-700">{error}</p>
            <button
              onClick={fetchUsers}
              className="px-4 py-2 bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors"
            >
              Retry
            </button>
          </div>
        )}

        {/* Users Table */}
        <div className="bg-white rounded-lg shadow overflow-hidden">
          <table role="table" className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  User
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Email
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Provider
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Status
                </th>
                <th
                  scope="col"
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {loading ? (
                <>
                  <UserSkeleton />
                  <UserSkeleton />
                  <UserSkeleton />
                  <UserSkeleton />
                  <UserSkeleton />
                </>
              ) : users.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-12 text-center text-gray-500">
                    No users found
                  </td>
                </tr>
              ) : (
                users.map((u) => <UserRow key={u.id} user={u} />)
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
