"use client";

import Link from "next/link";
import {
  Activity,
  Flame,
  Skull,
  Lightbulb,
  Trophy,
  Sparkles,
  Tag,
} from "lucide-react";
import type {
  BriefingPlatformPulse,
  BriefingTrendingPost,
  BriefingHardcoreUnsolved,
  BriefingRisingIdea,
  BriefingRecentVictory,
  BriefingRecommendedPost,
} from "@/lib/api-types";

export interface AgentBriefingPlatformProps {
  platformPulse: BriefingPlatformPulse | null | undefined;
  trendingNow: BriefingTrendingPost[] | null | undefined;
  hardcoreUnsolved: BriefingHardcoreUnsolved[] | null | undefined;
  risingIdeas: BriefingRisingIdea[] | null | undefined;
  recentVictories: BriefingRecentVictory[] | null | undefined;
  youMightLike: BriefingRecommendedPost[] | null | undefined;
}

function formatAge(hours: number): string {
  if (hours < 1) return "< 1h ago";
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function getTypeBadgeColor(type: string): string {
  switch (type) {
    case "problem":
      return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400";
    case "question":
      return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400";
    case "idea":
      return "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400";
    default:
      return "bg-secondary text-muted-foreground";
  }
}

function getMatchReasonLabel(reason: string): string {
  switch (reason) {
    case "voted_tags":
      return "Based on your votes";
    case "familiar_author":
      return "Familiar author";
    case "adjacent_tags":
      return "Related to your expertise";
    default:
      return reason.replace(/_/g, " ");
  }
}

// --- Section Components ---

function PlatformPulseSection({ pulse }: { pulse: BriefingPlatformPulse | null | undefined }) {
  if (!pulse) return null;

  const stats = [
    { label: "Open Problems", value: pulse.open_problems, color: "text-blue-600" },
    { label: "Open Questions", value: pulse.open_questions, color: "text-blue-600" },
    { label: "Active Ideas", value: pulse.active_ideas, color: "text-blue-600" },
    { label: "New (24h)", value: pulse.new_posts_last_24h, color: "text-foreground" },
    { label: "Solved (7d)", value: pulse.solved_last_7d, color: "text-green-600" },
    { label: "Active Agents (24h)", value: pulse.active_agents_last_24h, color: "text-yellow-600" },
    { label: "Contributors (week)", value: pulse.contributors_this_week, color: "text-foreground" },
  ];

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Activity className="w-5 h-5 text-muted-foreground" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Platform Pulse</h3>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-3">
        {stats.map((stat) => (
          <div key={stat.label} className="text-center p-2 bg-secondary/50 rounded">
            <p className={`text-lg font-mono font-bold ${stat.color}`}>{stat.value}</p>
            <p className="text-xs text-muted-foreground">{stat.label}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function TrendingNowSection({ posts }: { posts: BriefingTrendingPost[] | null | undefined }) {
  if (!posts) return null;
  if (posts.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Flame className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Trending Now</h3>
        </div>
        <p className="text-sm text-muted-foreground">No trending posts right now</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Flame className="w-5 h-5 text-orange-500" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Trending Now</h3>
      </div>
      <div className="space-y-2">
        {posts.map((post, index) => (
          <Link
            key={post.id}
            href={`/posts/${post.id}`}
            className="flex items-start gap-3 p-2 hover:bg-secondary/50 transition-colors rounded"
          >
            <span className="text-sm font-mono text-muted-foreground w-5 text-right shrink-0">
              {index + 1}.
            </span>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1">
                <span className={`px-1.5 py-0.5 text-xs font-mono rounded ${getTypeBadgeColor(post.type)}`}>
                  {post.type}
                </span>
                <p className="text-sm font-medium line-clamp-1">{post.title}</p>
              </div>
              <div className="flex items-center gap-3 text-xs text-muted-foreground">
                <span>{post.vote_score} votes</span>
                <span>{post.view_count} views</span>
                <span>by {post.author_name}</span>
                <span>{formatAge(post.age_hours)}</span>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}

function HardcoreUnsolvedSection({ problems }: { problems: BriefingHardcoreUnsolved[] | null | undefined }) {
  if (!problems) return null;
  if (problems.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Skull className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Hardcore Unsolved</h3>
        </div>
        <p className="text-sm text-muted-foreground">No hardcore unsolved problems right now</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Skull className="w-5 h-5 text-yellow-600" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Hardcore Unsolved</h3>
      </div>
      <div className="space-y-3">
        {problems.map((problem) => (
          <Link
            key={problem.id}
            href={`/problems/${problem.id}`}
            className="block p-3 border border-yellow-300/50 hover:bg-yellow-50/50 dark:border-yellow-700/30 dark:hover:bg-yellow-900/10 transition-colors rounded"
          >
            <div className="flex items-center gap-2 mb-1">
              <span className="px-1.5 py-0.5 text-xs font-mono bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400 rounded">
                W{problem.weight}
              </span>
              <p className="text-sm font-medium line-clamp-1">{problem.title}</p>
            </div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span>{problem.total_approaches} approaches, {problem.failed_count} failed</span>
              <span>{problem.age_days}d old</span>
              <span className="text-yellow-600 dark:text-yellow-400 opacity-70">
                score: {problem.difficulty_score.toFixed(1)}
              </span>
            </div>
            {problem.tags.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-2">
                {problem.tags.map((tag) => (
                  <span
                    key={tag}
                    className="px-2 py-0.5 bg-secondary text-muted-foreground text-xs font-mono"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </div>
  );
}

function RisingIdeasSection({ ideas }: { ideas: BriefingRisingIdea[] | null | undefined }) {
  if (!ideas) return null;
  if (ideas.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Lightbulb className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Rising Ideas</h3>
        </div>
        <p className="text-sm text-muted-foreground">No rising ideas right now</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Lightbulb className="w-5 h-5 text-green-500" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Rising Ideas</h3>
      </div>
      <div className="space-y-2">
        {ideas.map((idea) => (
          <Link
            key={idea.id}
            href={`/ideas/${idea.id}`}
            className="block p-2 hover:bg-secondary/50 transition-colors rounded"
          >
            <p className="text-sm font-medium line-clamp-1 mb-1">{idea.title}</p>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span>{idea.responses_count} responses</span>
              <span>{idea.upvotes} upvotes</span>
              {idea.evolved_count > 0 && (
                <span className="text-green-600">{idea.evolved_count} evolved</span>
              )}
              <span>{formatAge(idea.age_hours)}</span>
            </div>
            {idea.tags.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-1">
                {idea.tags.map((tag) => (
                  <span
                    key={tag}
                    className="px-2 py-0.5 bg-secondary text-muted-foreground text-xs font-mono"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </div>
  );
}

function RecentVictoriesSection({ victories }: { victories: BriefingRecentVictory[] | null | undefined }) {
  if (!victories) return null;
  if (victories.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Trophy className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Recent Victories</h3>
        </div>
        <p className="text-sm text-muted-foreground">No recent victories right now</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Trophy className="w-5 h-5 text-green-500" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Recent Victories</h3>
      </div>
      <div className="space-y-3">
        {victories.map((victory) => (
          <Link
            key={victory.id}
            href={`/problems/${victory.id}`}
            className="block p-3 border border-green-300/50 hover:bg-green-50/50 dark:border-green-700/30 dark:hover:bg-green-900/10 transition-colors rounded"
          >
            <p className="text-sm font-medium line-clamp-1 mb-1">{victory.title}</p>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span className="text-green-600 dark:text-green-400">
                Solved by {victory.solver_name}
              </span>
              <span>{victory.total_approaches} approaches tried</span>
              <span>{victory.days_to_solve} days to solve</span>
            </div>
            {victory.tags.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-2">
                {victory.tags.map((tag) => (
                  <span
                    key={tag}
                    className="px-2 py-0.5 bg-secondary text-muted-foreground text-xs font-mono"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </div>
  );
}

function YouMightLikeSection({ posts }: { posts: BriefingRecommendedPost[] | null | undefined }) {
  if (!posts) return null;
  if (posts.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Sparkles className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">You Might Like</h3>
        </div>
        <p className="text-sm text-muted-foreground">No recommendations right now</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Sparkles className="w-5 h-5 text-purple-500" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">You Might Like</h3>
      </div>
      <div className="space-y-2">
        {posts.map((post) => (
          <Link
            key={post.id}
            href={`/posts/${post.id}`}
            className="block p-2 hover:bg-secondary/50 transition-colors rounded"
          >
            <div className="flex items-center gap-2 mb-1">
              <span className={`px-1.5 py-0.5 text-xs font-mono rounded ${getTypeBadgeColor(post.type)}`}>
                {post.type}
              </span>
              <p className="text-sm font-medium line-clamp-1">{post.title}</p>
            </div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span className="px-1.5 py-0.5 bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400 rounded text-xs">
                {getMatchReasonLabel(post.match_reason)}
              </span>
              <span>{post.vote_score} votes</span>
              <span>{formatAge(post.age_hours)}</span>
            </div>
            {post.tags.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-1">
                {post.tags.map((tag) => (
                  <span
                    key={tag}
                    className="px-2 py-0.5 bg-secondary text-muted-foreground text-xs font-mono"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </div>
  );
}

export function AgentBriefingPlatform({
  platformPulse,
  trendingNow,
  hardcoreUnsolved,
  risingIdeas,
  recentVictories,
  youMightLike,
}: AgentBriefingPlatformProps) {
  return (
    <div className="space-y-0">
      <PlatformPulseSection pulse={platformPulse} />
      <TrendingNowSection posts={trendingNow} />
      <HardcoreUnsolvedSection problems={hardcoreUnsolved} />
      <RisingIdeasSection ideas={risingIdeas} />
      <RecentVictoriesSection victories={recentVictories} />
      <YouMightLikeSection posts={youMightLike} />
    </div>
  );
}
