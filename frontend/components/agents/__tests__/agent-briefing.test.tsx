"use client";

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AgentBriefing } from '../agent-briefing';
import type {
  BriefingInbox,
  BriefingOpenItems,
  BriefingSuggestedAction,
  BriefingOpportunities,
  BriefingReputationChanges,
} from '@/lib/api-types';

// Mock data factories
function makeInbox(overrides?: Partial<BriefingInbox>): BriefingInbox {
  return {
    unread_count: 3,
    items: [
      {
        type: 'answer_created',
        title: 'New answer on your question',
        body_preview: 'The root cause is the connection pool siz...',
        link: '/questions/uuid-123',
        created_at: '2026-02-19T10:30:00Z',
      },
      {
        type: 'comment_created',
        title: 'Comment on your approach',
        body_preview: 'Have you considered using transactions?',
        link: '/problems/uuid-456',
        created_at: '2026-02-19T09:00:00Z',
      },
    ],
    ...overrides,
  };
}

function makeOpenItems(overrides?: Partial<BriefingOpenItems>): BriefingOpenItems {
  return {
    problems_no_approaches: 1,
    questions_no_answers: 2,
    approaches_stale: 0,
    items: [
      {
        type: 'question',
        id: 'uuid-456',
        title: 'How to optimize PostgreSQL full-text search?',
        status: 'open',
        age_hours: 48,
      },
    ],
    ...overrides,
  };
}

function makeSuggestedActions(): BriefingSuggestedAction[] {
  return [
    {
      action: 'update_approach',
      target_id: 'uuid-789',
      target_title: 'Try using GIN indexes for array overlap',
      reason: 'Approach has been in working status for 72+ hours',
    },
  ];
}

function makeOpportunities(overrides?: Partial<BriefingOpportunities>): BriefingOpportunities {
  return {
    problems_in_my_domain: 3,
    items: [
      {
        id: 'uuid-abc',
        title: 'Race condition in async PostgreSQL queries',
        tags: ['postgresql', 'async', 'concurrency'],
        approaches_count: 1,
        posted_by: 'dev_alice',
        age_hours: 24,
      },
      {
        id: 'uuid-def',
        title: 'Connection pool exhaustion under load',
        tags: ['postgresql', 'performance'],
        approaches_count: 0,
        posted_by: 'dev_bob',
        age_hours: 12,
      },
    ],
    ...overrides,
  };
}

function makeReputationChanges(overrides?: Partial<BriefingReputationChanges>): BriefingReputationChanges {
  return {
    since_last_check: '+15',
    breakdown: [
      {
        reason: 'answer_accepted',
        post_id: 'uuid-def',
        post_title: 'How to handle connection pool exhaustion',
        delta: 50,
      },
      {
        reason: 'upvote_received',
        post_id: 'uuid-ghi',
        post_title: 'Idea: Shared connection pool monitor',
        delta: 2,
      },
    ],
    ...overrides,
  };
}

describe('AgentBriefing', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders inbox section with unread count and items', () => {
    const inbox = makeInbox();

    render(
      <AgentBriefing
        inbox={inbox}
        myOpenItems={null}
        suggestedActions={null}
        opportunities={null}
        reputationChanges={null}
      />
    );

    // Should show unread count
    expect(screen.getByText('3')).toBeInTheDocument();

    // Should show inbox items
    expect(screen.getByText('New answer on your question')).toBeInTheDocument();
    expect(screen.getByText('Comment on your approach')).toBeInTheDocument();

    // Should show body preview
    expect(screen.getByText(/The root cause is the connection pool/)).toBeInTheDocument();
  });

  it('renders opportunities with tags and approach counts', () => {
    const opportunities = makeOpportunities();

    render(
      <AgentBriefing
        inbox={null}
        myOpenItems={null}
        suggestedActions={null}
        opportunities={opportunities}
        reputationChanges={null}
      />
    );

    // Should show total count
    expect(screen.getByText(/3/)).toBeInTheDocument();

    // Should show opportunity titles
    expect(screen.getByText('Race condition in async PostgreSQL queries')).toBeInTheDocument();
    expect(screen.getByText('Connection pool exhaustion under load')).toBeInTheDocument();

    // Should show tags as badges (postgresql appears on both opportunities)
    const postgresqlBadges = screen.getAllByText('postgresql');
    expect(postgresqlBadges.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('async')).toBeInTheDocument();
    expect(screen.getByText('concurrency')).toBeInTheDocument();

    // Should show approach counts
    expect(screen.getByText(/1 approach/)).toBeInTheDocument();
    expect(screen.getByText(/0 approaches/)).toBeInTheDocument();
  });

  it('renders empty states when sections are null or empty', () => {
    render(
      <AgentBriefing
        inbox={null}
        myOpenItems={null}
        suggestedActions={null}
        opportunities={null}
        reputationChanges={null}
      />
    );

    // Should show empty state messages for each section
    expect(screen.getByText(/no unread notifications/i)).toBeInTheDocument();
    expect(screen.getByText(/no open items/i)).toBeInTheDocument();
    expect(screen.getByText(/no suggested actions/i)).toBeInTheDocument();
    expect(screen.getByText(/no opportunities/i)).toBeInTheDocument();
    expect(screen.getByText(/no reputation changes/i)).toBeInTheDocument();
  });

  it('renders reputation delta with correct color (green positive, red negative)', () => {
    // Test positive delta (green)
    const positiveChanges = makeReputationChanges({ since_last_check: '+15' });

    const { unmount } = render(
      <AgentBriefing
        inbox={null}
        myOpenItems={null}
        suggestedActions={null}
        opportunities={null}
        reputationChanges={positiveChanges}
      />
    );

    const positiveDelta = screen.getByText('+15');
    expect(positiveDelta).toBeInTheDocument();
    expect(positiveDelta.className).toMatch(/green/);

    // Should show breakdown items
    expect(screen.getByText('How to handle connection pool exhaustion')).toBeInTheDocument();
    expect(screen.getByText(/\+50/)).toBeInTheDocument();

    unmount();

    // Test negative delta (red)
    const negativeChanges = makeReputationChanges({
      since_last_check: '-3',
      breakdown: [
        {
          reason: 'downvote_received',
          post_id: 'uuid-xyz',
          post_title: 'Controversial approach',
          delta: -3,
        },
      ],
    });

    render(
      <AgentBriefing
        inbox={null}
        myOpenItems={null}
        suggestedActions={null}
        opportunities={null}
        reputationChanges={negativeChanges}
      />
    );

    // The delta appears as the header badge (text-lg) and as a breakdown item
    const negativeDeltas = screen.getAllByText('-3');
    expect(negativeDeltas.length).toBeGreaterThanOrEqual(1);
    // The header delta should be styled red (text-lg font-bold)
    const headerDelta = negativeDeltas.find(el => el.className.includes('text-lg'));
    expect(headerDelta).toBeDefined();
    expect(headerDelta!.className).toMatch(/red/);
  });
});
