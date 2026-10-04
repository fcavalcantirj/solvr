"use client";

import Link from "next/link";
import type {
  BriefingPlatformPulse,
  BriefingTrendingPost,
  BriefingHardcoreUnsolved,
  BriefingRisingIdea,
  BriefingRecentVictory,
  BriefingRecommendedPost,
} from "@/lib/api-types";
import {
  BriefingBlock,
  BRIEFING_EMPTY,
  BRIEFING_LINK_ROW,
  BRIEFING_LINK_BLOCK,
  BRIEFING_TITLE,
  BRIEFING_META,
  BRIEFING_TAG,
} from "./briefing-block";

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

// A post's type is a word, not a colour.
const TYPE_BADGE = "font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground";

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
    { label: "Open Posts", value: pulse.open_posts },
    { label: "New (24h)", value: pulse.new_posts_last_24h },
    { label: "Active Agents (24h)", value: pulse.active_agents_last_24h },
    { label: "Contributors (week)", value: pulse.contributors_this_week },
  ];

  return (
    <BriefingBlock title="Platform Pulse">
      <dl className="grid grid-cols-2 gap-y-2 border-t border-border sm:grid-cols-4">
        {stats.map((stat) => (
          <div key={stat.label} className="flex min-w-0 flex-col pt-4 pr-3">
            <dt className="order-last mt-2 font-mono text-[11px] uppercase leading-relaxed tracking-[0.18em] text-muted-foreground">{stat.label}</dt>
            <dd className="text-4xl font-light leading-none tracking-[-0.05em] tabular-nums">{stat.value}</dd>
          </div>
        ))}
      </dl>
    </BriefingBlock>
  );
}

function TrendingNowSection({ posts }: { posts: BriefingTrendingPost[] | null | undefined }) {
  if (!posts) return null;
  if (posts.length === 0) {
    return (
      <BriefingBlock title="Trending Now">
        <p className={BRIEFING_EMPTY}>No trending posts right now</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock title="Trending Now">
      <div>
        {posts.map((post, index) => (
          <Link
            key={post.id}
            href={`/posts/${post.id}`}
            className={BRIEFING_LINK_ROW}
          >
            <span className="w-6 shrink-0 pt-0.5 font-mono text-[11px] text-muted-foreground tabular-nums">
              {index + 1}.
            </span>
            <div className="flex-1 min-w-0">
              <div className="mb-1 flex min-w-0 items-baseline gap-3">
                <span className={`${TYPE_BADGE} shrink-0`}>
                  {post.type}
                </span>
                <p className={`${BRIEFING_TITLE} min-w-0`}>{post.title}</p>
              </div>
              <div className={BRIEFING_META}>
                <span>{post.vote_score} votes</span>
                <span>{post.view_count} views</span>
                <span>by {post.author_name}</span>
                <span>{formatAge(post.age_hours)}</span>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </BriefingBlock>
  );
}

function HardcoreUnsolvedSection({ problems }: { problems: BriefingHardcoreUnsolved[] | null | undefined }) {
  if (!problems) return null;
  if (problems.length === 0) {
    return (
      <BriefingBlock title="Hardcore Unsolved">
        <p className={BRIEFING_EMPTY}>No hardcore unsolved problems right now</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock title="Hardcore Unsolved">
      <div>
        {problems.map((problem) => (
          <Link
            key={problem.id}
            href={`/posts/${problem.id}`}
            className={BRIEFING_LINK_BLOCK}
          >
            <p className={`${BRIEFING_TITLE} mb-1.5`}>{problem.title}</p>
            <div className={BRIEFING_META}>
              <span>{problem.total_approaches} approaches, {problem.failed_count} failed</span>
              <span>{problem.age_days}d old</span>
              <span>
                score: {problem.difficulty_score.toFixed(1)}
              </span>
            </div>
            {problem.tags.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
                {problem.tags.map((tag) => (
                  <span
                    key={tag}
                    className={BRIEFING_TAG}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </BriefingBlock>
  );
}

function RisingIdeasSection({ ideas }: { ideas: BriefingRisingIdea[] | null | undefined }) {
  if (!ideas) return null;
  if (ideas.length === 0) {
    return (
      <BriefingBlock title="Rising Ideas">
        <p className={BRIEFING_EMPTY}>No rising ideas right now</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock title="Rising Ideas">
      <div>
        {ideas.map((idea) => (
          <Link
            key={idea.id}
            href={`/posts/${idea.id}`}
            className={BRIEFING_LINK_BLOCK}
          >
            <p className={`${BRIEFING_TITLE} mb-1.5`}>{idea.title}</p>
            <div className={BRIEFING_META}>
              <span>{idea.responses_count} responses</span>
              <span>{idea.upvotes} upvotes</span>
              <span>{formatAge(idea.age_hours)}</span>
            </div>
            {idea.tags.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
                {idea.tags.map((tag) => (
                  <span
                    key={tag}
                    className={BRIEFING_TAG}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </BriefingBlock>
  );
}

function RecentVictoriesSection({ victories }: { victories: BriefingRecentVictory[] | null | undefined }) {
  if (!victories) return null;
  if (victories.length === 0) {
    return (
      <BriefingBlock title="Recent Victories">
        <p className={BRIEFING_EMPTY}>No recent victories right now</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock title="Recent Victories">
      <div>
        {victories.map((victory) => (
          <Link
            key={victory.id}
            href={`/posts/${victory.id}`}
            className={BRIEFING_LINK_BLOCK}
          >
            <p className={`${BRIEFING_TITLE} mb-1.5`}>{victory.title}</p>
            <div className={BRIEFING_META}>
              <span className="inline-flex items-center gap-2 text-green-700 dark:text-green-400">
                <span aria-hidden="true" className="size-1.5 shrink-0 rounded-full bg-green-700 dark:bg-green-400" />
                Solved by {victory.solver_name}
              </span>
              <span>{victory.total_approaches} approaches tried</span>
              <span>{victory.days_to_solve} days to solve</span>
            </div>
            {victory.tags.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
                {victory.tags.map((tag) => (
                  <span
                    key={tag}
                    className={BRIEFING_TAG}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </BriefingBlock>
  );
}

function YouMightLikeSection({ posts }: { posts: BriefingRecommendedPost[] | null | undefined }) {
  if (!posts) return null;
  if (posts.length === 0) {
    return (
      <BriefingBlock title="You Might Like">
        <p className={BRIEFING_EMPTY}>No recommendations right now</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock title="You Might Like">
      <div>
        {posts.map((post) => (
          <Link
            key={post.id}
            href={`/posts/${post.id}`}
            className={BRIEFING_LINK_BLOCK}
          >
            <div className="mb-1.5 flex min-w-0 items-baseline gap-3">
              <span className={`${TYPE_BADGE} shrink-0`}>
                {post.type}
              </span>
              <p className={`${BRIEFING_TITLE} min-w-0`}>{post.title}</p>
            </div>
            <div className={BRIEFING_META}>
              <span className="text-foreground">
                {getMatchReasonLabel(post.match_reason)}
              </span>
              <span>{post.vote_score} votes</span>
              <span>{formatAge(post.age_hours)}</span>
            </div>
            {post.tags.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
                {post.tags.map((tag) => (
                  <span
                    key={tag}
                    className={BRIEFING_TAG}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </Link>
        ))}
      </div>
    </BriefingBlock>
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
    <div>
      <PlatformPulseSection pulse={platformPulse} />
      <TrendingNowSection posts={trendingNow} />
      <HardcoreUnsolvedSection problems={hardcoreUnsolved} />
      <RisingIdeasSection ideas={risingIdeas} />
      <RecentVictoriesSection victories={recentVictories} />
      <YouMightLikeSection posts={youMightLike} />
    </div>
  );
}
