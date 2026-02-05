'use client';

/**
 * Admin User Detail Page
 * Per SPEC.md Part 16.3 and PRD lines 524-525
 *
 * Features:
 * - Requires admin role (admin or super_admin)
 * - User detail with profile info
 * - User activity timeline
 * - Admin action buttons: warn, suspend, ban
 */

import { useEffect, useState, useCallback } from 'react';
import { useRouter, useParams } from 'next/navigation';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/lib/api';

// Types for user data
interface Activity {
  id: string;
  type: string;
  description: string;
  created_at: string;
}

interface UserDetail {
  id: string;
  username: string;
  display_name: string;
  email: string;
  avatar_url?: string;
  bio?: string;
  auth_provider: string;
  role: string;
  status: string;
  created_at: string;
  updated_at: string;
  activity?: Activity[];
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

// Format date for display
function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

// Loading skeleton for user detail
function UserDetailSkeleton() {
  return (
    <div data-testid="user-detail-skeleton" className="animate-pulse">
      <div className="flex items-start gap-6">
        <div className="w-24 h-24 rounded-full bg-gray-200"></div>
        <div className="flex-1">
          <div className="h-8 bg-gray-200 rounded w-48 mb-2"></div>
          <div className="h-4 bg-gray-200 rounded w-32 mb-4"></div>
          <div className="h-4 bg-gray-200 rounded w-64 mb-2"></div>
          <div className="h-4 bg-gray-200 rounded w-48"></div>
        </div>
      </div>
    </div>
  );
}

// Activity item component
interface ActivityItemProps {
  activity: Activity;
}

function ActivityItem({ activity }: ActivityItemProps) {
  return (
    <div className="flex items-start gap-3 py-3 border-b border-gray-100 last:border-0">
      <div className="w-2 h-2 rounded-full bg-blue-500 mt-2 flex-shrink-0"></div>
      <div className="flex-1 min-w-0">
        <p className="text-sm text-gray-900">{activity.description}</p>
        <p className="text-xs text-gray-500 mt-1">{formatDate(activity.created_at)}</p>
      </div>
    </div>
  );
}

export default function AdminUserDetailPage() {
  const router = useRouter();
  const params = useParams();
  const userId = params.id as string;
  const { user: authUser, isLoading: authLoading } = useAuth();

  const [userDetail, setUserDetail] = useState<UserDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [showBanConfirm, setShowBanConfirm] = useState(false);

  // Fetch user detail
  const fetchUserDetail = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await api.get<UserDetail>(`/v1/admin/users/${userId}`);
      setUserDetail(response);
    } catch (err) {
      if ((err as { status?: number }).status === 404) {
        setError('User not found');
      } else {
        setError('Failed to load user details');
      }
      console.error('User detail fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, [userId]);

  // Check auth and fetch user detail
  useEffect(() => {
    if (authLoading) return;

    if (!authUser) {
      router.replace('/login');
      return;
    }

    if (!isAdminRole(authUser.role)) {
      router.replace('/');
      return;
    }

    fetchUserDetail();
  }, [authUser, authLoading, router, fetchUserDetail]);

  // Handle warn action
  const handleWarn = async () => {
    try {
      setActionLoading(true);
      setActionError(null);
      setActionSuccess(null);

      await api.post(`/v1/admin/users/${userId}/warn`, { message: 'Warning from admin' });

      setActionSuccess('Warning sent successfully');
    } catch (err) {
      setActionError('Action failed. Please try again.');
      console.error('Warn error:', err);
    } finally {
      setActionLoading(false);
    }
  };

  // Handle suspend action
  const handleSuspend = async () => {
    try {
      setActionLoading(true);
      setActionError(null);
      setActionSuccess(null);

      await api.post(`/v1/admin/users/${userId}/suspend`, {
        duration: '7d',
        reason: 'Admin action',
      });

      // Update local state
      if (userDetail) {
        setUserDetail({ ...userDetail, status: 'suspended' });
      }
      setActionSuccess('User suspended successfully');
    } catch (err) {
      setActionError('Action failed. Please try again.');
      console.error('Suspend error:', err);
    } finally {
      setActionLoading(false);
    }
  };

  // Handle ban action
  const handleBan = async () => {
    try {
      setActionLoading(true);
      setActionError(null);
      setActionSuccess(null);
      setShowBanConfirm(false);

      await api.post(`/v1/admin/users/${userId}/ban`, { reason: 'Admin action' });

      // Update local state
      if (userDetail) {
        setUserDetail({ ...userDetail, status: 'banned' });
      }
      setActionSuccess('User banned successfully');
    } catch (err) {
      setActionError('Action failed. Please try again.');
      console.error('Ban error:', err);
    } finally {
      setActionLoading(false);
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
  if (!authUser || !isAdminRole(authUser.role)) {
    return null;
  }

  return (
    <main className="min-h-screen bg-gray-50 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <Link
            href="/admin/users"
            className="text-sm text-blue-600 hover:text-blue-800 mb-2 inline-block"
          >
            &larr; Back to Users
          </Link>
          <h1 className="text-3xl font-bold text-gray-900">User Details</h1>
        </div>

        {/* Error State */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-6 flex items-center justify-between">
            <p className="text-red-700">{error}</p>
            <button
              onClick={fetchUserDetail}
              className="px-4 py-2 bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors"
            >
              Retry
            </button>
          </div>
        )}

        {/* Loading State */}
        {loading && !error && <UserDetailSkeleton />}

        {/* User Content */}
        {!loading && !error && userDetail && (
          <>
            {/* Action Messages */}
            {actionSuccess && (
              <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-6">
                <p className="text-green-700">{actionSuccess}</p>
              </div>
            )}
            {actionError && (
              <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-6">
                <p className="text-red-700">{actionError}</p>
              </div>
            )}

            {/* User Profile Card */}
            <div className="bg-white rounded-lg shadow p-6 mb-6">
              <div className="flex items-start gap-6">
                {/* Avatar */}
                {userDetail.avatar_url ? (
                  <img
                    src={userDetail.avatar_url}
                    alt={userDetail.display_name}
                    className="w-24 h-24 rounded-full object-cover"
                  />
                ) : (
                  <div className="w-24 h-24 rounded-full bg-gray-200 flex items-center justify-center">
                    <span className="text-gray-500 text-3xl">
                      {userDetail.display_name.charAt(0).toUpperCase()}
                    </span>
                  </div>
                )}

                {/* User Info */}
                <div className="flex-1">
                  <div className="flex items-start justify-between">
                    <div>
                      <h2 className="text-2xl font-bold text-gray-900">
                        {userDetail.display_name}
                      </h2>
                      <p className="text-gray-500">@{userDetail.username}</p>
                    </div>
                    <span
                      className={`px-3 py-1 rounded-full text-sm font-medium capitalize ${
                        statusColors[userDetail.status] || 'bg-gray-100 text-gray-800'
                      }`}
                    >
                      {userDetail.status}
                    </span>
                  </div>

                  {userDetail.bio && (
                    <p className="text-gray-700 mt-3">{userDetail.bio}</p>
                  )}

                  <div className="grid grid-cols-2 gap-4 mt-4 text-sm">
                    <div>
                      <span className="text-gray-500">Email:</span>
                      <p className="text-gray-900">{userDetail.email}</p>
                    </div>
                    <div>
                      <span className="text-gray-500">Provider:</span>
                      <p className="text-gray-900 capitalize">{userDetail.auth_provider}</p>
                    </div>
                    <div>
                      <span className="text-gray-500">Role:</span>
                      <p className="text-gray-900 capitalize">{userDetail.role}</p>
                    </div>
                    <div>
                      <span className="text-gray-500">Joined:</span>
                      <p className="text-gray-900">{formatDate(userDetail.created_at)}</p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Admin Actions */}
              <div className="mt-6 pt-6 border-t border-gray-200">
                <h3 className="text-sm font-medium text-gray-700 mb-3">Admin Actions</h3>
                <div className="flex gap-3">
                  <button
                    onClick={handleWarn}
                    disabled={actionLoading}
                    className="px-4 py-2 bg-yellow-100 text-yellow-700 rounded-md hover:bg-yellow-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    Warn
                  </button>
                  <button
                    onClick={handleSuspend}
                    disabled={actionLoading || userDetail.status === 'suspended'}
                    className="px-4 py-2 bg-orange-100 text-orange-700 rounded-md hover:bg-orange-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    Suspend
                  </button>
                  {showBanConfirm ? (
                    <div className="flex items-center gap-2">
                      <span className="text-sm text-red-600">Are you sure?</span>
                      <button
                        onClick={handleBan}
                        disabled={actionLoading}
                        className="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setShowBanConfirm(false)}
                        className="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 transition-colors"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setShowBanConfirm(true)}
                      disabled={actionLoading || userDetail.status === 'banned'}
                      className="px-4 py-2 bg-red-100 text-red-700 rounded-md hover:bg-red-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                      Ban
                    </button>
                  )}
                </div>
              </div>
            </div>

            {/* Activity Section */}
            <div className="bg-white rounded-lg shadow p-6">
              <h2 className="text-lg font-semibold text-gray-900 mb-4">Recent Activity</h2>

              {!userDetail.activity || userDetail.activity.length === 0 ? (
                <p className="text-gray-500 text-center py-8">No activity found</p>
              ) : (
                <div className="divide-y divide-gray-100">
                  {userDetail.activity.map((act) => (
                    <ActivityItem key={act.id} activity={act} />
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </main>
  );
}
