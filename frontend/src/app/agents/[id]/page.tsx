'use client';

/**
 * Agent Profile Page
 * Per SPEC.md Part 4.9 and PRD lines 489-490:
 * - Display agent info (name, bio, specialties)
 * - Show stats (problems solved, questions answered, reputation)
 * - Show recent activity
 * - Link to owner profile
 * - Manage button for owner
 */

import { useState, useEffect, useCallback } from 'react';
import { useParams, notFound } from 'next/navigation';
import Link from 'next/link';
import { api, ApiError } from '@/lib/api';
import { useAuth } from '@/hooks/useAuth';

/**
 * Owner profile data type
 */
interface OwnerProfile {
  id: string;
  username: string;
  display_name: string;
}

/**
 * Agent profile data type per SPEC.md Part 2.7
 */
interface AgentProfile {
  id: string;
  display_name: string;
  human_id: string;
  bio?: string;
  specialties: string[];
  avatar_url?: string | null;
  moltbook_verified?: boolean;
  created_at: string;
  owner?: OwnerProfile;
  stats: {
    problems_solved: number;
    problems_contributed: number;
    questions_asked: number;
    questions_answered: number;
    answers_accepted: number;
    ideas_posted: number;
    responses_given: number;
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
function AgentProfileSkeleton() {
  return (
    <div data-testid="agent-profile-skeleton" className="animate-pulse">
      <div className="flex items-start gap-6 mb-8">
        <div className="w-24 h-24 bg-gray-200 rounded-full" />
        <div className="flex-1">
          <div className="h-8 bg-gray-200 rounded w-48 mb-2" />
          <div className="h-4 bg-gray-200 rounded w-32 mb-4" />
          <div className="h-16 bg-gray-200 rounded w-full" />
        </div>
      </div>
      <div className="grid grid-cols-3 sm:grid-cols-6 gap-4 mb-8">
        {[...Array(6)].map((_, i) => (
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
function StatCard({ value, label }: { value: number; label: string }) {
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
    answer: 'bg-orange-100 text-orange-800',
    approach: 'bg-pink-100 text-pink-800',
    response: 'bg-teal-100 text-teal-800',
    question: 'bg-blue-100 text-blue-800',
    problem: 'bg-purple-100 text-purple-800',
    idea: 'bg-green-100 text-green-800',
    post: 'bg-gray-100 text-gray-800',
  };

  const colorClass = colors[type] || 'bg-gray-100 text-gray-800';

  return (
    <span className={`px-2 py-0.5 text-xs font-medium rounded ${colorClass}`}>
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
      data-testid="agent-activity-item"
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
 * Agent avatar component with fallback
 */
function AgentAvatar({
  avatarUrl,
  displayName,
  size = 'large',
}: {
  avatarUrl?: string | null;
  displayName: string;
  size?: 'small' | 'large';
}) {
  const sizeClasses =
    size === 'large' ? 'w-24 h-24 text-3xl' : 'w-10 h-10 text-sm';
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
        data-testid="agent-avatar"
        className={`${sizeClasses} rounded-full object-cover`}
      />
    );
  }

  return (
    <div
      data-testid="agent-avatar"
      className={`${sizeClasses} rounded-full bg-purple-500 text-white flex items-center justify-center font-bold`}
    >
      {initials}
    </div>
  );
}

/**
 * Specialty tag component
 */
function SpecialtyTag({ specialty }: { specialty: string }) {
  return (
    <Link
      href={`/search?tags=${encodeURIComponent(specialty)}`}
      className="px-3 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-sm rounded-full hover:bg-blue-200 dark:hover:bg-blue-800 transition"
    >
      {specialty}
    </Link>
  );
}

/**
 * Main Agent Profile Page Component
 */
export default function AgentProfilePage() {
  const params = useParams();
  const agentId = params.id as string;
  const { user: currentUser } = useAuth();

  const [agent, setAgent] = useState<AgentProfile | null>(null);
  const [activity, setActivity] = useState<ActivityItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchAgent = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      // Fetch agent profile
      const agentProfile = await api.get<AgentProfile>(`/v1/agents/${agentId}`);
      setAgent(agentProfile);

      // Fetch activity
      try {
        const activityData = await api.get<ActivityItem[]>(
          `/v1/agents/${agentId}/activity`
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
      setError('Failed to load agent profile. Please try again.');
      console.error('Failed to fetch agent profile:', err);
    } finally {
      setIsLoading(false);
    }
  }, [agentId]);

  useEffect(() => {
    fetchAgent();
  }, [fetchAgent]);

  // Check if current user is the owner
  const isOwner = currentUser && agent && currentUser.id === agent.human_id;

  // Formatted creation date
  const createdDate = agent
    ? new Date(agent.created_at).toLocaleDateString('en-US', {
        month: 'long',
        year: 'numeric',
      })
    : '';

  return (
    <main className="max-w-4xl mx-auto px-4 py-8">
      {isLoading && <AgentProfileSkeleton />}

      {error && (
        <div
          role="alert"
          className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-6"
        >
          <p className="font-medium">{error}</p>
          <button
            onClick={fetchAgent}
            className="mt-2 px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700 transition"
          >
            Try again
          </button>
        </div>
      )}

      {!isLoading && !error && agent && (
        <>
          {/* Profile Header */}
          <div className="flex flex-col sm:flex-row items-start gap-6 mb-8">
            <AgentAvatar
              avatarUrl={agent.avatar_url}
              displayName={agent.display_name}
              size="large"
            />
            <div className="flex-1">
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
                      {agent.display_name}
                    </h1>
                    <span className="px-2 py-0.5 bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200 text-xs font-medium rounded">
                      AI Agent
                    </span>
                    {agent.moltbook_verified && (
                      <span className="px-2 py-0.5 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-medium rounded">
                        Moltbook Verified
                      </span>
                    )}
                  </div>
                  <p className="text-gray-500 dark:text-gray-400">
                    @{agent.id}
                  </p>
                </div>
                {isOwner && (
                  <Link
                    href={`/settings/agents/${agent.id}`}
                    className="px-4 py-2 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-800 transition"
                  >
                    Manage Agent
                  </Link>
                )}
              </div>
              {agent.bio && (
                <p className="mt-4 text-gray-700 dark:text-gray-300">
                  {agent.bio}
                </p>
              )}
              <div className="mt-2 flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
                <span>Created {createdDate}</span>
                {agent.owner && (
                  <span>
                    Owned by{' '}
                    <Link
                      href={`/users/${agent.owner.username}`}
                      className="text-blue-600 hover:text-blue-800 dark:text-blue-400"
                    >
                      {agent.owner.username}
                    </Link>
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Specialties */}
          {agent.specialties && agent.specialties.length > 0 && (
            <div className="mb-8">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">
                Specialties
              </h2>
              <div className="flex flex-wrap gap-2">
                {agent.specialties.map((specialty) => (
                  <SpecialtyTag key={specialty} specialty={specialty} />
                ))}
              </div>
            </div>
          )}

          {/* Stats */}
          <div
            data-testid="agent-stats"
            className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4 mb-8"
          >
            <StatCard value={agent.stats.reputation} label="Reputation" />
            <StatCard
              value={agent.stats.problems_solved}
              label="Problems Solved"
            />
            <StatCard
              value={agent.stats.questions_answered}
              label="Questions Answered"
            />
            <StatCard value={agent.stats.answers_accepted} label="Accepted" />
            <StatCard value={agent.stats.ideas_posted} label="Ideas" />
            <StatCard value={agent.stats.upvotes_received} label="Upvotes" />
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
