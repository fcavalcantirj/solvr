/**
 * Tests for VoteButtons component
 * Per PRD requirement: Create VoteButtons with upvote/downvote buttons,
 * show current score, API integration, and optimistic updates
 */

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// Import after mocks
import VoteButtons from '../components/VoteButtons';

describe('VoteButtons', () => {
  describe('basic rendering', () => {
    it('renders upvote button', () => {
      render(<VoteButtons score={10} targetId="post-123" targetType="post" />);
      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      expect(upvoteButton).toBeInTheDocument();
    });

    it('renders downvote button', () => {
      render(<VoteButtons score={10} targetId="post-123" targetType="post" />);
      const downvoteButton = screen.getByRole('button', { name: /downvote/i });
      expect(downvoteButton).toBeInTheDocument();
    });

    it('displays current score', () => {
      render(<VoteButtons score={42} targetId="post-123" targetType="post" />);
      expect(screen.getByText('42')).toBeInTheDocument();
    });

    it('displays zero score', () => {
      render(<VoteButtons score={0} targetId="post-123" targetType="post" />);
      expect(screen.getByText('0')).toBeInTheDocument();
    });

    it('displays negative score', () => {
      render(<VoteButtons score={-5} targetId="post-123" targetType="post" />);
      expect(screen.getByText('-5')).toBeInTheDocument();
    });
  });

  describe('user vote state', () => {
    it('highlights upvote button when user has upvoted', () => {
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" userVote="up" />
      );
      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      expect(upvoteButton).toHaveAttribute('data-active', 'true');
    });

    it('highlights downvote button when user has downvoted', () => {
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" userVote="down" />
      );
      const downvoteButton = screen.getByRole('button', { name: /downvote/i });
      expect(downvoteButton).toHaveAttribute('data-active', 'true');
    });

    it('neither button is highlighted when user has not voted', () => {
      render(<VoteButtons score={10} targetId="post-123" targetType="post" />);
      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      const downvoteButton = screen.getByRole('button', { name: /downvote/i });
      expect(upvoteButton).not.toHaveAttribute('data-active', 'true');
      expect(downvoteButton).not.toHaveAttribute('data-active', 'true');
    });
  });

  describe('vote interaction', () => {
    it('calls onVote with "up" when upvote is clicked', async () => {
      const onVote = jest.fn();
      const user = userEvent.setup();
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" onVote={onVote} />
      );

      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      await user.click(upvoteButton);

      expect(onVote).toHaveBeenCalledWith('up');
    });

    it('calls onVote with "down" when downvote is clicked', async () => {
      const onVote = jest.fn();
      const user = userEvent.setup();
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" onVote={onVote} />
      );

      const downvoteButton = screen.getByRole('button', { name: /downvote/i });
      await user.click(downvoteButton);

      expect(onVote).toHaveBeenCalledWith('down');
    });

    it('calls onVote with null to remove vote when clicking active upvote', async () => {
      const onVote = jest.fn();
      const user = userEvent.setup();
      render(
        <VoteButtons
          score={10}
          targetId="post-123"
          targetType="post"
          userVote="up"
          onVote={onVote}
        />
      );

      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      await user.click(upvoteButton);

      expect(onVote).toHaveBeenCalledWith(null);
    });

    it('calls onVote with null to remove vote when clicking active downvote', async () => {
      const onVote = jest.fn();
      const user = userEvent.setup();
      render(
        <VoteButtons
          score={10}
          targetId="post-123"
          targetType="post"
          userVote="down"
          onVote={onVote}
        />
      );

      const downvoteButton = screen.getByRole('button', { name: /downvote/i });
      await user.click(downvoteButton);

      expect(onVote).toHaveBeenCalledWith(null);
    });
  });

  describe('optimistic updates', () => {
    it('updates score optimistically on upvote', async () => {
      const user = userEvent.setup();
      render(
        <VoteButtons
          score={10}
          targetId="post-123"
          targetType="post"
          onVote={jest.fn()}
        />
      );

      expect(screen.getByText('10')).toBeInTheDocument();

      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      await user.click(upvoteButton);

      // Should immediately show +1
      expect(screen.getByText('11')).toBeInTheDocument();
    });

    it('updates score optimistically on downvote', async () => {
      const user = userEvent.setup();
      render(
        <VoteButtons
          score={10}
          targetId="post-123"
          targetType="post"
          onVote={jest.fn()}
        />
      );

      const downvoteButton = screen.getByRole('button', { name: /downvote/i });
      await user.click(downvoteButton);

      // Should immediately show -1
      expect(screen.getByText('9')).toBeInTheDocument();
    });

    it('changes score by 2 when switching from up to down', async () => {
      const user = userEvent.setup();
      render(
        <VoteButtons
          score={10}
          targetId="post-123"
          targetType="post"
          userVote="up"
          onVote={jest.fn()}
        />
      );

      const downvoteButton = screen.getByRole('button', { name: /downvote/i });
      await user.click(downvoteButton);

      // Should change from +1 to -1, net -2
      expect(screen.getByText('8')).toBeInTheDocument();
    });

    it('reverts optimistic update when removing upvote', async () => {
      const user = userEvent.setup();
      render(
        <VoteButtons
          score={11}
          targetId="post-123"
          targetType="post"
          userVote="up"
          onVote={jest.fn()}
        />
      );

      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      await user.click(upvoteButton);

      // Should decrease by 1 when removing upvote
      expect(screen.getByText('10')).toBeInTheDocument();
    });
  });

  describe('loading state', () => {
    it('disables buttons while loading', () => {
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" loading />
      );
      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      const downvoteButton = screen.getByRole('button', { name: /downvote/i });

      expect(upvoteButton).toBeDisabled();
      expect(downvoteButton).toBeDisabled();
    });
  });

  describe('disabled state', () => {
    it('disables buttons when disabled prop is true', () => {
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" disabled />
      );
      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      const downvoteButton = screen.getByRole('button', { name: /downvote/i });

      expect(upvoteButton).toBeDisabled();
      expect(downvoteButton).toBeDisabled();
    });

    it('does not call onVote when disabled', async () => {
      const onVote = jest.fn();
      const user = userEvent.setup();
      render(
        <VoteButtons
          score={10}
          targetId="post-123"
          targetType="post"
          disabled
          onVote={onVote}
        />
      );

      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      await user.click(upvoteButton);

      expect(onVote).not.toHaveBeenCalled();
    });
  });

  describe('layout variants', () => {
    it('renders vertical layout by default', () => {
      render(<VoteButtons score={10} targetId="post-123" targetType="post" />);
      const container = screen.getByRole('button', { name: /upvote/i }).parentElement;
      expect(container).toHaveClass('flex-col');
    });

    it('renders horizontal layout when specified', () => {
      render(
        <VoteButtons
          score={10}
          targetId="post-123"
          targetType="post"
          layout="horizontal"
        />
      );
      const container = screen.getByRole('button', { name: /upvote/i }).parentElement;
      expect(container).toHaveClass('flex-row');
    });
  });

  describe('accessibility', () => {
    it('buttons have accessible names', () => {
      render(<VoteButtons score={10} targetId="post-123" targetType="post" />);
      expect(screen.getByRole('button', { name: /upvote/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /downvote/i })).toBeInTheDocument();
    });

    it('indicates current vote state to screen readers', () => {
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" userVote="up" />
      );
      const upvoteButton = screen.getByRole('button', { name: /upvote/i });
      expect(upvoteButton).toHaveAttribute('aria-pressed', 'true');
    });
  });

  describe('size variants', () => {
    it('renders small size', () => {
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" size="sm" />
      );
      const container = screen.getByRole('button', { name: /upvote/i }).parentElement;
      expect(container).toBeInTheDocument();
    });

    it('renders large size', () => {
      render(
        <VoteButtons score={10} targetId="post-123" targetType="post" size="lg" />
      );
      const container = screen.getByRole('button', { name: /upvote/i }).parentElement;
      expect(container).toBeInTheDocument();
    });
  });
});
