"use client";

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ProfileHeader } from './profile-header';
import type { UserData } from '@/hooks/use-user';

const mockUser: UserData = {
  id: 'user-123',
  username: 'johndoe',
  displayName: 'John Doe',
  avatarUrl: 'https://example.com/avatar.png',
  bio: 'A passionate developer working on awesome projects.',
  stats: {
    postsCreated: 42,
    contributions: 150,
    karma: 1337,
  },
};

describe('ProfileHeader', () => {
  it('should render user display name', () => {
    render(<ProfileHeader user={mockUser} />);

    expect(screen.getByText('John Doe')).toBeInTheDocument();
  });

  it('should render username', () => {
    render(<ProfileHeader user={mockUser} />);

    expect(screen.getByText('@johndoe')).toBeInTheDocument();
  });

  it('should render bio', () => {
    render(<ProfileHeader user={mockUser} />);

    expect(screen.getByText('A passionate developer working on awesome projects.')).toBeInTheDocument();
  });

  it('should render user stats', () => {
    render(<ProfileHeader user={mockUser} />);

    // Posts created
    expect(screen.getByText('42')).toBeInTheDocument();
    expect(screen.getByText(/posts/i)).toBeInTheDocument();

    // Contributions
    expect(screen.getByText('150')).toBeInTheDocument();
    expect(screen.getByText(/contributions/i)).toBeInTheDocument();

    // Karma
    expect(screen.getByText('1337')).toBeInTheDocument();
    expect(screen.getByText(/karma/i)).toBeInTheDocument();
  });

  it('should handle user without avatar', () => {
    const userWithoutAvatar: UserData = {
      ...mockUser,
      avatarUrl: undefined,
    };

    render(<ProfileHeader user={userWithoutAvatar} />);

    // Should still render without crashing
    expect(screen.getByText('John Doe')).toBeInTheDocument();
  });

  it('should handle user without bio', () => {
    const userWithoutBio: UserData = {
      ...mockUser,
      bio: undefined,
    };

    render(<ProfileHeader user={userWithoutBio} />);

    // Should still render without crashing
    expect(screen.getByText('John Doe')).toBeInTheDocument();
  });

  it('should handle zero stats', () => {
    const userWithZeroStats: UserData = {
      ...mockUser,
      stats: {
        postsCreated: 0,
        contributions: 0,
        karma: 0,
      },
    };

    render(<ProfileHeader user={userWithZeroStats} />);

    // Should render zeros
    const zeros = screen.getAllByText('0');
    expect(zeros.length).toBeGreaterThanOrEqual(3);
  });
});
