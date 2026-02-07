"use client";

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AgentActivityFeed } from './agent-activity-feed';
import * as useAgentActivityModule from '@/hooks/use-agent-activity';

// Mock the hook
vi.mock('@/hooks/use-agent-activity', () => ({
  useAgentActivity: vi.fn(),
}));

describe('AgentActivityFeed', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getActivityLink - correct URL routing', () => {
    it('links problems to /problems/{id}', () => {
      // Arrange
      vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
        items: [{
          id: 'problem-123',
          type: 'post',
          postType: 'problem',
          action: 'created',
          title: 'Test Problem',
          createdAt: '2025-01-01T00:00:00Z',
          time: '1d ago',
        }],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Act
      render(<AgentActivityFeed agentId="agent_test" />);

      // Assert - link should be /problems/problem-123, NOT /posts/problem-123
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/problems/problem-123');
    });

    it('links questions to /questions/{id}', () => {
      // Arrange
      vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
        items: [{
          id: 'question-456',
          type: 'post',
          postType: 'question',
          action: 'created',
          title: 'Test Question',
          createdAt: '2025-01-01T00:00:00Z',
          time: '1d ago',
        }],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Act
      render(<AgentActivityFeed agentId="agent_test" />);

      // Assert
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/questions/question-456');
    });

    it('links ideas to /ideas/{id}', () => {
      // Arrange
      vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
        items: [{
          id: 'idea-789',
          type: 'post',
          postType: 'idea',
          action: 'created',
          title: 'Test Idea',
          createdAt: '2025-01-01T00:00:00Z',
          time: '1d ago',
        }],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Act
      render(<AgentActivityFeed agentId="agent_test" />);

      // Assert
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/ideas/idea-789');
    });

    it('links approaches to /problems/{targetId}', () => {
      // Arrange - approaches are on problems
      vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
        items: [{
          id: 'approach-001',
          type: 'approach',
          action: 'started_approach',
          title: 'My Approach',
          createdAt: '2025-01-01T00:00:00Z',
          time: '1d ago',
          targetId: 'problem-parent',
          targetTitle: 'Parent Problem',
        }],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Act
      render(<AgentActivityFeed agentId="agent_test" />);

      // Assert - approaches link to their parent problem
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/problems/problem-parent');
    });

    it('links answers to /questions/{targetId}', () => {
      // Arrange - answers are on questions
      vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
        items: [{
          id: 'answer-001',
          type: 'answer',
          action: 'answered',
          title: 'My Answer',
          createdAt: '2025-01-01T00:00:00Z',
          time: '1d ago',
          targetId: 'question-parent',
          targetTitle: 'Parent Question',
        }],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Act
      render(<AgentActivityFeed agentId="agent_test" />);

      // Assert - answers link to their parent question
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/questions/question-parent');
    });

    it('links responses to /ideas/{targetId}', () => {
      // Arrange - responses are on ideas
      vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
        items: [{
          id: 'response-001',
          type: 'response',
          action: 'responded',
          title: 'My Response',
          createdAt: '2025-01-01T00:00:00Z',
          time: '1d ago',
          targetId: 'idea-parent',
          targetTitle: 'Parent Idea',
        }],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Act
      render(<AgentActivityFeed agentId="agent_test" />);

      // Assert - responses link to their parent idea
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/ideas/idea-parent');
    });
  });

  it('shows loading state', () => {
    vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
      items: [],
      loading: true,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<AgentActivityFeed agentId="agent_test" />);

    // Loader2 has animate-spin class
    expect(document.querySelector('.animate-spin')).toBeInTheDocument();
  });

  it('shows error state', () => {
    vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
      items: [],
      loading: false,
      error: 'Failed to load',
      total: 0,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<AgentActivityFeed agentId="agent_test" />);

    expect(screen.getByText('Failed to load')).toBeInTheDocument();
  });

  it('shows empty state', () => {
    vi.mocked(useAgentActivityModule.useAgentActivity).mockReturnValue({
      items: [],
      loading: false,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<AgentActivityFeed agentId="agent_test" />);

    expect(screen.getByText('No activity yet')).toBeInTheDocument();
  });
});
