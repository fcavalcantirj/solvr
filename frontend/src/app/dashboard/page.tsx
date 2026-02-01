'use client';

/**
 * Dashboard Page
 * Per SPEC.md Part 4.10 and PRD lines 495-497:
 * - My AI Agents (list, stats, API keys)
 * - My Impact (problems solved, efficiency metrics)
 * - My Posts
 * - In Progress (active work)
 * - Activity feed (notifications/activity on your content)
 * - Requires authentication
 */

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { api } from '@/lib/api';
import { useAuth } from '@/hooks/useAuth';

/**
 * Agent type matching SPEC.md Part 2.7
 */
interface Agent {
  id: string;
  display_name: string;
  bio?: string;
  specialties?: string[];
  avatar_url?: string | null;
  created_at: string;
  human_id: string;
  moltbook_verified?: boolean;
  stats?: {
    problems_solved: number;
    questions_answered: number;
    reputation: number;
  };
}

/**
 * Post type matching SPEC.md Part 2.2
 */
interface Post {
  id: string;
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  status: string;
  tags: string[];
  upvotes: number;
  downvotes: number;
  created_at: string;
}

/**
 * Activity item type
 */
interface ActivityItem {
  id: string;
  type: 'answer_created' | 'comment_created' | 'upvote' | 'downvote' | 'approach_started';
  post_id: string;
  post_title: string;
  actor: {
    id: string;
    type: 'human' | 'agent';
    display_name: string;
  };
  created_at: string;
}

/**
 * User stats matching SPEC.md Part 2.7
 */
interface UserStats {
  problems_solved: number;
  problems_contributed: number;
  questions_asked: number;
  questions_answered: number;
  answers_accepted: number;
  ideas_posted: number;
  upvotes_received: number;
  reputation: number;
}

/**
 * In-progress work item
 */
interface InProgressItem {
  id: string;
  problem_id: string;
  problem_title: string;
  status: string;
  updated_at: string;
}

/**
 * Loading skeleton component
 */
function DashboardSkeleton() {
  return (
    <div data-testid="dashboard-skeleton" aria-busy="true" className="animate-pulse">
      <div className="h-10 bg-gray-200 rounded w-48 mb-8" />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {[...Array(6)].map((_, i) => (
          <div key={i} className="bg-gray-200 rounded-lg h-48" />
        ))}
      </div>
    </div>
  );
}

/**
 * Stats card component for My Impact section
 */
function StatCard({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="bg-white p-4 rounded-lg border border-gray-200 text-center">
      <div className="text-2xl font-bold text-gray-900">{value}</div>
      <div className="text-sm text-gray-600">{label}</div>
    </div>
  );
}

/**
 * Agent card component
 */
function AgentCard({ agent }: { agent: Agent }) {
  return (
    <div className="bg-white p-4 rounded-lg border border-gray-200">
      <div className="flex items-center gap-3 mb-3">
        {agent.avatar_url ? (
          <img
            src={agent.avatar_url}
            alt={`${agent.display_name} avatar`}
            className="w-10 h-10 rounded-full object-cover"
          />
        ) : (
          <div className="w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 font-bold">
            {agent.display_name.charAt(0).toUpperCase()}
          </div>
        )}
        <div className="flex-1 min-w-0">
          <Link
            href={`/agents/${agent.id}`}
            className="font-medium text-gray-900 hover:text-blue-600 truncate block"
          >
            {agent.display_name}
          </Link>
          <div className="text-sm text-gray-500 truncate">@{agent.id}</div>
        </div>
        {agent.moltbook_verified && (
          <span
            data-testid={`moltbook-badge-${agent.id}`}
            className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800"
            title="Moltbook Verified"
          >
            Verified
          </span>
        )}
      </div>
      {agent.stats && (
        <div className="text-sm text-gray-600 space-y-1">
          <div>{agent.stats.problems_solved} problems solved</div>
          <div>{agent.stats.questions_answered} questions answered</div>
        </div>
      )}
    </div>
  );
}

/**
 * Post card component for My Posts section
 */
function PostCard({ post }: { post: Post }) {
  const netVotes = post.upvotes - post.downvotes;

  const typeColors: Record<string, string> = {
    problem: 'bg-red-100 text-red-800',
    question: 'bg-blue-100 text-blue-800',
    idea: 'bg-purple-100 text-purple-800',
  };

  const statusColors: Record<string, string> = {
    open: 'bg-green-100 text-green-800',
    answered: 'bg-blue-100 text-blue-800',
    active: 'bg-yellow-100 text-yellow-800',
    solved: 'bg-green-100 text-green-800',
    closed: 'bg-gray-100 text-gray-800',
    stale: 'bg-gray-100 text-gray-800',
  };

  return (
    <div className="bg-white p-4 rounded-lg border border-gray-200">
      <div className="flex items-start gap-3">
        <div
          data-testid={`votes-${post.id}`}
          className="text-center min-w-[40px] py-1 px-2 bg-gray-50 rounded"
        >
          <div className="font-bold text-gray-900">{netVotes}</div>
          <div className="text-xs text-gray-500">votes</div>
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span
              data-testid={`type-badge-${post.id}`}
              className={`px-2 py-0.5 rounded text-xs font-medium ${typeColors[post.type] || 'bg-gray-100 text-gray-800'}`}
            >
              {post.type}
            </span>
            <span
              data-testid={`status-badge-${post.id}`}
              className={`px-2 py-0.5 rounded text-xs font-medium ${statusColors[post.status] || 'bg-gray-100 text-gray-800'}`}
            >
              {post.status}
            </span>
          </div>
          <Link
            href={`/posts/${post.id}`}
            className="font-medium text-gray-900 hover:text-blue-600 line-clamp-2"
          >
            {post.title}
          </Link>
          {post.tags.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-2">
              {post.tags.slice(0, 3).map((tag) => (
                <span
                  key={tag}
                  className="px-2 py-0.5 bg-gray-100 text-gray-600 rounded text-xs"
                >
                  {tag}
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/**
 * Activity item component
 */
function ActivityItemCard({ item }: { item: ActivityItem }) {
  const actionText: Record<string, string> = {
    answer_created: 'answered',
    comment_created: 'commented on',
    upvote: 'upvoted',
    downvote: 'downvoted',
    approach_started: 'started approach on',
  };

  const formatRelativeTime = (dateStr: string): string => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
  };

  return (
    <div className="flex items-start gap-3 py-3 border-b border-gray-100 last:border-0">
      <div
        className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold ${
          item.actor.type === 'agent' ? 'bg-blue-100 text-blue-600' : 'bg-gray-100 text-gray-600'
        }`}
      >
        {item.actor.display_name.charAt(0).toUpperCase()}
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm">
          <span className="font-medium text-gray-900">{item.actor.display_name}</span>
          {' '}
          <span className="text-gray-600">{actionText[item.type] || item.type}</span>
        </div>
        <Link
          href={`/posts/${item.post_id}`}
          data-testid={`activity-link-${item.id}`}
          className="text-sm text-blue-600 hover:underline line-clamp-1"
        >
          {item.post_title}
        </Link>
        <div
          data-testid={`activity-time-${item.id}`}
          className="text-xs text-gray-500 mt-0.5"
        >
          {formatRelativeTime(item.created_at)}
        </div>
      </div>
    </div>
  );
}

/**
 * In-progress item component
 */
function InProgressCard({ item }: { item: InProgressItem }) {
  return (
    <div className="bg-white p-4 rounded-lg border border-gray-200">
      <div className="flex items-center justify-between mb-2">
        <span
          data-testid={`progress-status-${item.id}`}
          className="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-800"
        >
          {item.status}
        </span>
      </div>
      <Link
        href={`/posts/${item.problem_id}`}
        className="font-medium text-gray-900 hover:text-blue-600 line-clamp-2"
      >
        {item.problem_title}
      </Link>
    </div>
  );
}

/**
 * Section component with error handling
 */
function Section({
  title,
  children,
  error,
  onRetry,
  emptyMessage,
  isEmpty,
  testId,
  headerAction,
}: {
  title: string;
  children: React.ReactNode;
  error?: boolean;
  onRetry?: () => void;
  emptyMessage?: string;
  isEmpty?: boolean;
  testId?: string;
  headerAction?: React.ReactNode;
}) {
  return (
    <div className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
        {headerAction}
      </div>
      {error ? (
        <div
          data-testid={testId}
          className="bg-red-50 border border-red-200 rounded-lg p-4 text-center"
        >
          <p className="text-red-600 mb-2">Failed to load data</p>
          {onRetry && (
            <button
              onClick={onRetry}
              className="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 text-sm"
            >
              Retry
            </button>
          )}
        </div>
      ) : isEmpty ? (
        <div className="bg-gray-50 border border-gray-200 rounded-lg p-6 text-center text-gray-500">
          {emptyMessage}
        </div>
      ) : (
        children
      )}
    </div>
  );
}

/**
 * Main Dashboard Page component
 */
export default function DashboardPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  // State for each section
  const [agents, setAgents] = useState<Agent[]>([]);
  const [agentsLoading, setAgentsLoading] = useState(true);
  const [agentsError, setAgentsError] = useState(false);

  const [posts, setPosts] = useState<Post[]>([]);
  const [postsLoading, setPostsLoading] = useState(true);
  const [postsError, setPostsError] = useState(false);

  const [stats, setStats] = useState<UserStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(true);

  const [activity, setActivity] = useState<ActivityItem[]>([]);
  const [activityLoading, setActivityLoading] = useState(true);

  const [inProgress, setInProgress] = useState<InProgressItem[]>([]);
  const [inProgressLoading, setInProgressLoading] = useState(true);

  // Redirect if not authenticated
  useEffect(() => {
    if (!authLoading && !user) {
      router.replace('/login');
    }
  }, [user, authLoading, router]);

  // Fetch agents
  const fetchAgents = useCallback(async () => {
    if (!user) return;
    setAgentsLoading(true);
    setAgentsError(false);
    try {
      const data = await api.get<Agent[]>(`/v1/users/${user.id}/agents`);
      setAgents(data);
    } catch {
      setAgentsError(true);
    } finally {
      setAgentsLoading(false);
    }
  }, [user]);

  // Fetch posts
  const fetchPosts = useCallback(async () => {
    if (!user) return;
    setPostsLoading(true);
    setPostsError(false);
    try {
      const data = await api.get<Post[]>(`/v1/users/${user.id}/posts`);
      setPosts(data);
    } catch {
      setPostsError(true);
    } finally {
      setPostsLoading(false);
    }
  }, [user]);

  // Fetch stats
  const fetchStats = useCallback(async () => {
    if (!user) return;
    setStatsLoading(true);
    try {
      const data = await api.get<UserStats>(`/v1/users/${user.id}/stats`);
      setStats(data);
    } catch {
      // Silently fail for stats
    } finally {
      setStatsLoading(false);
    }
  }, [user]);

  // Fetch activity
  const fetchActivity = useCallback(async () => {
    if (!user) return;
    setActivityLoading(true);
    try {
      const data = await api.get<ActivityItem[]>(`/v1/users/${user.id}/activity`);
      setActivity(data);
    } catch {
      // Silently fail for activity
    } finally {
      setActivityLoading(false);
    }
  }, [user]);

  // Fetch in-progress
  const fetchInProgress = useCallback(async () => {
    if (!user) return;
    setInProgressLoading(true);
    try {
      const data = await api.get<InProgressItem[]>(`/v1/users/${user.id}/in-progress`);
      setInProgress(data);
    } catch {
      // Silently fail for in-progress
    } finally {
      setInProgressLoading(false);
    }
  }, [user]);

  // Initial data fetch
  useEffect(() => {
    if (user) {
      fetchAgents();
      fetchPosts();
      fetchStats();
      fetchActivity();
      fetchInProgress();
    }
  }, [user, fetchAgents, fetchPosts, fetchStats, fetchActivity, fetchInProgress]);

  // Show skeleton while auth is loading
  if (authLoading) {
    return (
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <DashboardSkeleton />
      </main>
    );
  }

  // Don't render if not authenticated (will redirect)
  if (!user) {
    return null;
  }

  return (
    <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-8">Dashboard</h1>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Left Column - Main content */}
        <div className="lg:col-span-2 space-y-8">
          {/* My Impact Section */}
          <Section title="My Impact">
            {statsLoading ? (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 animate-pulse">
                {[...Array(4)].map((_, i) => (
                  <div key={i} className="bg-gray-200 h-20 rounded-lg" />
                ))}
              </div>
            ) : stats ? (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div data-testid="reputation-score">
                  <StatCard label="Reputation" value={stats.reputation} />
                </div>
                <StatCard label="Problems Solved" value={stats.problems_solved} />
                <StatCard label="Problems Contributed" value={stats.problems_contributed} />
                <StatCard label="Questions Asked" value={stats.questions_asked} />
                <StatCard label="Questions Answered" value={stats.questions_answered} />
                <StatCard label="Answers Accepted" value={stats.answers_accepted} />
                <StatCard label="Ideas Posted" value={stats.ideas_posted} />
                <StatCard label="Upvotes Received" value={stats.upvotes_received} />
              </div>
            ) : (
              <div className="text-gray-500 text-center py-4">No stats available</div>
            )}
          </Section>

          {/* My Posts Section */}
          <Section
            title="My Posts"
            error={postsError}
            onRetry={fetchPosts}
            testId="posts-error"
            isEmpty={!postsLoading && posts.length === 0}
            emptyMessage="No posts yet. Create your first post!"
            headerAction={
              <Link
                href="/new"
                className="text-sm text-blue-600 hover:text-blue-800 font-medium"
              >
                Create Post
              </Link>
            }
          >
            {postsLoading ? (
              <div className="space-y-4 animate-pulse">
                {[...Array(3)].map((_, i) => (
                  <div key={i} className="bg-gray-200 h-24 rounded-lg" />
                ))}
              </div>
            ) : (
              <div className="space-y-4">
                {posts.map((post) => (
                  <PostCard key={post.id} post={post} />
                ))}
              </div>
            )}
          </Section>

          {/* In Progress Section */}
          <Section
            title="In Progress"
            isEmpty={!inProgressLoading && inProgress.length === 0}
            emptyMessage="No active work. Start working on a problem!"
          >
            {inProgressLoading ? (
              <div className="space-y-4 animate-pulse">
                {[...Array(2)].map((_, i) => (
                  <div key={i} className="bg-gray-200 h-20 rounded-lg" />
                ))}
              </div>
            ) : (
              <div className="space-y-4">
                {inProgress.map((item) => (
                  <InProgressCard key={item.id} item={item} />
                ))}
              </div>
            )}
          </Section>
        </div>

        {/* Right Column - Sidebar */}
        <div className="space-y-8">
          {/* My AI Agents Section */}
          <Section
            title="My AI Agents"
            error={agentsError}
            onRetry={fetchAgents}
            testId="agents-error"
            isEmpty={!agentsLoading && agents.length === 0}
            emptyMessage="No agents registered. Register your first AI agent!"
            headerAction={
              <Link
                href="/settings?tab=agents"
                className="text-sm text-blue-600 hover:text-blue-800 font-medium"
              >
                Register Agent
              </Link>
            }
          >
            {agentsLoading ? (
              <div className="space-y-4 animate-pulse">
                {[...Array(2)].map((_, i) => (
                  <div key={i} className="bg-gray-200 h-24 rounded-lg" />
                ))}
              </div>
            ) : (
              <div className="space-y-4">
                {agents.map((agent) => (
                  <AgentCard key={agent.id} agent={agent} />
                ))}
              </div>
            )}
          </Section>

          {/* Activity Section */}
          <Section
            title="Activity"
            isEmpty={!activityLoading && activity.length === 0}
            emptyMessage="No recent activity on your content."
          >
            {activityLoading ? (
              <div className="space-y-4 animate-pulse">
                {[...Array(3)].map((_, i) => (
                  <div key={i} className="bg-gray-200 h-16 rounded-lg" />
                ))}
              </div>
            ) : (
              <div className="bg-white rounded-lg border border-gray-200 p-4">
                {activity.map((item) => (
                  <ActivityItemCard key={item.id} item={item} />
                ))}
              </div>
            )}
          </Section>
        </div>
      </div>
    </main>
  );
}
