'use client';

/**
 * User Profile Page
 * Per SPEC.md Part 4.9 and PRD lines 485-488:
 * - Display user info (name, bio, avatar)
 * - Show stats (posts, answers, reputation)
 * - Show recent activity
 * - Edit button for own profile
 */

import { useState, useEffect, useCallback } from 'react';
import { useParams, notFound } from 'next/navigation';
import Link from 'next/link';
import { api, ApiError } from '@/lib/api';
import { useAuth } from '@/hooks/useAuth';

/**
 * User profile data type
 */
interface UserProfile {
  id: string;
  username: string;
  display_name: string;
  email?: string;
  avatar_url?: string | null;
  bio?: string;
  created_at: string;
  agents_count?: number;
  stats: {
    posts_created: number;
    answers_given: number;
    answers_accepted: number;
    upvotes_received: number;
    reputation: number;
  };
}

/**
 * Activity item type
 */
interface ActivityItem {
  id: string;
  type: 'post' | 'answer' | 'approach' | 'response';
  action: string;
  title: string;
  post_type?: string;
  status?: string;
  created_at: string;
  target_id: string;
  target_title?: string;
}

/**
 * Loading skeleton component
 */
function ProfileSkeleton() {
  return (
    <div data-testid="profile-skeleton" className="animate-pulse">
      <div className="flex items-start gap-6 mb-8">
        <div className="w-24 h-24 bg-gray-200 rounded-full" />
        <div className="flex-1">
          <div className="h-8 bg-gray-200 rounded w-48 mb-2" />
          <div className="h-4 bg-gray-200 rounded w-32 mb-4" />
          <div className="h-16 bg-gray-200 rounded w-full" />
        </div>
      </div>
      <div className="grid grid-cols-4 gap-4 mb-8">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="bg-gray-100 p-4 rounded-lg">
            <div className="h-8 bg-gray-200 rounded w-16 mb-2" />
            <div className="h-4 bg-gray-200 rounded w-20" />
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * Stats card component
 */
function StatCard({
  value,
  label,
}: {
  value: number;
  label: string;
}) {
  const formattedValue = value.toLocaleString();

  return (
    <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg text-center">
      <div className="text-2xl font-bold text-gray-900 dark:text-white">
        {formattedValue}
      </div>
      <div className="text-sm text-gray-600 dark:text-gray-400">{label}</div>
    </div>
  );
}

/**
 * Activity type badge
 */
function ActivityTypeBadge({ type }: { type: string }) {
  const colors: Record<string, string> = {
    question: 'bg-blue-100 text-blue-800',
    problem: 'bg-purple-100 text-purple-800',
    idea: 'bg-green-100 text-green-800',
    answer: 'bg-orange-100 text-orange-800',
    approach: 'bg-pink-100 text-pink-800',
    response: 'bg-teal-100 text-teal-800',
  };

  const colorClass = colors[type] || 'bg-gray-100 text-gray-800';

  return (
    <span
      className={`px-2 py-0.5 text-xs font-medium rounded ${colorClass}`}
    >
      {type}
    </span>
  );
}

/**
 * Activity item component
 */
function ActivityItemCard({ activity }: { activity: ActivityItem }) {
  const formattedDate = new Date(activity.created_at).toLocaleDateString(
    'en-US',
    {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    }
  );

  // Determine the link based on activity type
  let href = '#';
  if (activity.type === 'post') {
    href = `/posts/${activity.target_id}`;
  } else if (activity.type === 'answer') {
    href = `/posts/${activity.target_id}`;
  } else if (activity.type === 'approach') {
    href = `/posts/${activity.target_id}`;
  }

  return (
    <article
      data-testid="activity-item"
      className="flex items-start gap-4 py-4 border-b border-gray-200 dark:border-gray-700 last:border-0"
    >
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <ActivityTypeBadge type={activity.post_type || activity.type} />
          <span className="text-sm text-gray-500">{formattedDate}</span>
        </div>
        <Link
          href={href}
          className="text-gray-900 dark:text-white hover:text-blue-600 font-medium line-clamp-2"
        >
          {activity.title}
        </Link>
        {activity.status && (
          <span className="text-sm text-gray-500 mt-1 block">
            Status: {activity.status}
          </span>
        )}
      </div>
    </article>
  );
}

/**
 * User avatar component with fallback
 */
function UserAvatar({
  avatarUrl,
  displayName,
  size = 'large',
}: {
  avatarUrl?: string | null;
  displayName: string;
  size?: 'small' | 'large';
}) {
  const sizeClasses = size === 'large' ? 'w-24 h-24 text-3xl' : 'w-10 h-10 text-sm';
  const initials = displayName
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);

  if (avatarUrl) {
    return (
      <img
        src={avatarUrl}
        alt={displayName}
        data-testid="user-avatar"
        className={`${sizeClasses} rounded-full object-cover`}
      />
    );
  }

  return (
    <div
      data-testid="user-avatar"
      className={`${sizeClasses} rounded-full bg-blue-500 text-white flex items-center justify-center font-bold`}
    >
      {initials}
    </div>
  );
}

/**
 * Main User Profile Page Component
 */
export default function UserProfilePage() {
  const params = useParams();
  const username = params.username as string;
  const { user: currentUser } = useAuth();

  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [activity, setActivity] = useState<ActivityItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchProfile = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      // Fetch user profile
      const userProfile = await api.get<UserProfile>(`/v1/users/${username}`);
      setProfile(userProfile);

      // Fetch activity
      try {
        const activityData = await api.get<ActivityItem[]>(
          `/v1/users/${username}/activity`
        );
        setActivity(activityData);
      } catch (activityError) {
        // Activity fetch failure is not critical
        console.error('Failed to fetch activity:', activityError);
        setActivity([]);
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        notFound();
        return;
      }
      setError('Failed to load user profile. Please try again.');
      console.error('Failed to fetch user profile:', err);
    } finally {
      setIsLoading(false);
    }
  }, [username]);

  useEffect(() => {
    fetchProfile();
  }, [fetchProfile]);

  // Check if viewing own profile
  const isOwnProfile = currentUser && currentUser.username === username;

  // Formatted join date
  const joinedDate = profile
    ? new Date(profile.created_at).toLocaleDateString('en-US', {
        month: 'long',
        year: 'numeric',
      })
    : '';

  return (
    <main className="max-w-4xl mx-auto px-4 py-8">
      {isLoading && <ProfileSkeleton />}

      {error && (
        <div
          role="alert"
          className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-6"
        >
          <p className="font-medium">{error}</p>
          <button
            onClick={fetchProfile}
            className="mt-2 px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700 transition"
          >
            Try again
          </button>
        </div>
      )}

      {!isLoading && !error && profile && (
        <>
          {/* Profile Header */}
          <div className="flex flex-col sm:flex-row items-start gap-6 mb-8">
            <UserAvatar
              avatarUrl={profile.avatar_url}
              displayName={profile.display_name}
              size="large"
            />
            <div className="flex-1">
              <div className="flex items-start justify-between">
                <div>
                  <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
                    {profile.display_name}
                  </h1>
                  <p className="text-gray-500 dark:text-gray-400">
                    @{profile.username}
                  </p>
                </div>
                {isOwnProfile && (
                  <Link
                    href="/settings"
                    className="px-4 py-2 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-800 transition"
                  >
                    Edit Profile
                  </Link>
                )}
              </div>
              {profile.bio && (
                <p className="mt-4 text-gray-700 dark:text-gray-300">
                  {profile.bio}
                </p>
              )}
              <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                Joined {joinedDate}
              </p>
              {profile.agents_count && profile.agents_count > 0 && (
                <Link
                  href={`/users/${username}/agents`}
                  className="inline-block mt-2 text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400"
                >
                  {profile.agents_count} agents
                </Link>
              )}
            </div>
          </div>

          {/* Stats */}
          <div data-testid="user-stats" className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8">
            <StatCard value={profile.stats.reputation} label="Reputation" />
            <StatCard value={profile.stats.posts_created} label="Posts" />
            <StatCard value={profile.stats.answers_given} label="Answers" />
            <StatCard value={profile.stats.answers_accepted} label="Accepted" />
          </div>

          {/* Activity Section */}
          <section>
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
              Recent Activity
            </h2>
            {activity.length === 0 ? (
              <p className="text-gray-500 dark:text-gray-400 py-8 text-center">
                No recent activity
              </p>
            ) : (
              <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 divide-y divide-gray-200 dark:divide-gray-700">
                {activity.map((item) => (
                  <ActivityItemCard key={item.id} activity={item} />
                ))}
              </div>
            )}
          </section>
        </>
      )}
    </main>
  );
}
