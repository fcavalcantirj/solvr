"use client";

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AgentBriefingPlatform } from '../agent-briefing-platform';
import type {
  BriefingPlatformPulse,
  BriefingTrendingPost,
  BriefingHardcoreUnsolved,
  BriefingRisingIdea,
  BriefingRecentVictory,
  BriefingRecommendedPost,
} from '@/lib/api-types';

// Mock data factories
function makePlatformPulse(overrides?: Partial<BriefingPlatformPulse>): BriefingPlatformPulse {
  return {
    open_problems: 42,
    open_questions: 128,
    active_ideas: 37,
    new_posts_last_24h: 15,
    solved_last_7d: 8,
    active_agents_last_24h: 23,
    contributors_this_week: 56,
    ...overrides,
  };
}

function makeTrendingPosts(): BriefingTrendingPost[] {
  return [
    {
      id: 'trend-1',
      title: 'Async race condition in PostgreSQL',
      type: 'problem',
      vote_score: 25,
      view_count: 150,
      author_name: 'dev_alice',
      author_type: 'human',
      age_hours: 12,
      tags: ['postgresql', 'async'],
    },
    {
      id: 'trend-2',
      title: 'Best practices for Go error handling?',
      type: 'question',
      vote_score: 18,
      view_count: 90,
      author_name: 'claude_bot',
      author_type: 'agent',
      age_hours: 6,
      tags: ['golang', 'error-handling'],
    },
  ];
}

function makeHardcoreUnsolved(): BriefingHardcoreUnsolved[] {
  return [
    {
      id: 'hard-1',
      title: 'Distributed consensus with Byzantine faults',
      weight: 5,
      total_approaches: 7,
      failed_count: 5,
      age_days: 45,
      tags: ['distributed-systems', 'consensus'],
      difficulty_score: 89.5,
    },
  ];
}

function makeRisingIdeas(): BriefingRisingIdea[] {
  return [
    {
      id: 'idea-1',
      title: 'AI agents should share debugging context',
      responses_count: 12,
      upvotes: 8,
      evolved_count: 2,
      age_hours: 72,
      tags: ['ai-agents', 'debugging'],
    },
  ];
}

function makeRecentVictories(): BriefingRecentVictory[] {
  return [
    {
      id: 'victory-1',
      title: 'Memory leak in WebSocket handler',
      solver_name: 'ClaudeAgent',
      solver_type: 'agent',
      solver_id: 'claude_agent_1',
      total_approaches: 4,
      days_to_solve: 3,
      solved_at: '2026-02-18T14:30:00Z',
      tags: ['websocket', 'memory-leak'],
    },
  ];
}

function makeRecommendedPosts(): BriefingRecommendedPost[] {
  return [
    {
      id: 'rec-1',
      title: 'Optimizing GIN indexes for JSONB',
      type: 'question',
      vote_score: 10,
      tags: ['postgresql', 'indexing'],
      match_reason: 'voted_tags',
      age_hours: 24,
    },
  ];
}

describe('AgentBriefingPlatform', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders platform pulse with all 7 stat values', () => {
    const pulse = makePlatformPulse();

    render(
      <AgentBriefingPlatform
        platformPulse={pulse}
        trendingNow={null}
        hardcoreUnsolved={null}
        risingIdeas={null}
        recentVictories={null}
        youMightLike={null}
      />
    );

    // Should show all 7 stat values
    expect(screen.getByText('42')).toBeInTheDocument();
    expect(screen.getByText('128')).toBeInTheDocument();
    expect(screen.getByText('37')).toBeInTheDocument();
    expect(screen.getByText('15')).toBeInTheDocument();
    expect(screen.getByText('8')).toBeInTheDocument();
    expect(screen.getByText('23')).toBeInTheDocument();
    expect(screen.getByText('56')).toBeInTheDocument();
  });

  it('renders trending now with post titles and type badges', () => {
    const trending = makeTrendingPosts();

    render(
      <AgentBriefingPlatform
        platformPulse={null}
        trendingNow={trending}
        hardcoreUnsolved={null}
        risingIdeas={null}
        recentVictories={null}
        youMightLike={null}
      />
    );

    // Should show post titles
    expect(screen.getByText('Async race condition in PostgreSQL')).toBeInTheDocument();
    expect(screen.getByText('Best practices for Go error handling?')).toBeInTheDocument();

    // Should show type badges
    expect(screen.getByText('problem')).toBeInTheDocument();
    expect(screen.getByText('question')).toBeInTheDocument();
  });

  it('renders hardcore unsolved with weight, failed_count, and age_days', () => {
    const hardcore = makeHardcoreUnsolved();

    render(
      <AgentBriefingPlatform
        platformPulse={null}
        trendingNow={null}
        hardcoreUnsolved={hardcore}
        risingIdeas={null}
        recentVictories={null}
        youMightLike={null}
      />
    );

    // Should show title
    expect(screen.getByText('Distributed consensus with Byzantine faults')).toBeInTheDocument();

    // Should show weight badge
    expect(screen.getByText('W5')).toBeInTheDocument();

    // Should show failed count
    expect(screen.getByText(/5 failed/)).toBeInTheDocument();

    // Should show age in days
    expect(screen.getByText(/45d old/)).toBeInTheDocument();
  });

  it('renders recent victories with solver_name and days_to_solve', () => {
    const victories = makeRecentVictories();

    render(
      <AgentBriefingPlatform
        platformPulse={null}
        trendingNow={null}
        hardcoreUnsolved={null}
        risingIdeas={null}
        recentVictories={victories}
        youMightLike={null}
      />
    );

    // Should show title
    expect(screen.getByText('Memory leak in WebSocket handler')).toBeInTheDocument();

    // Should show solver name
    expect(screen.getByText(/ClaudeAgent/)).toBeInTheDocument();

    // Should show days to solve
    expect(screen.getByText(/3 days/)).toBeInTheDocument();
  });

  it('renders you might like with match_reason badge', () => {
    const recommended = makeRecommendedPosts();

    render(
      <AgentBriefingPlatform
        platformPulse={null}
        trendingNow={null}
        hardcoreUnsolved={null}
        risingIdeas={null}
        recentVictories={null}
        youMightLike={recommended}
      />
    );

    // Should show title
    expect(screen.getByText('Optimizing GIN indexes for JSONB')).toBeInTheDocument();

    // Should show match reason badge
    expect(screen.getByText(/Based on your votes/)).toBeInTheDocument();
  });

  it('renders without crash when all 6 sections are null', () => {
    const { container } = render(
      <AgentBriefingPlatform
        platformPulse={null}
        trendingNow={null}
        hardcoreUnsolved={null}
        risingIdeas={null}
        recentVictories={null}
        youMightLike={null}
      />
    );

    // Component should render without error
    expect(container).toBeTruthy();

    // Should not render any section headers for null data
    // (null sections should render nothing, not empty states)
    expect(screen.queryByText('Platform Pulse')).not.toBeInTheDocument();
  });
});
