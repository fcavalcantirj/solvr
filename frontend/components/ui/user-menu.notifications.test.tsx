import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';

vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({ user: { id: 'u1', displayName: 'Me', type: 'human' }, logout: vi.fn() }),
}));
vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

import { UserMenu } from './user-menu';

describe('UserMenu notifications (idx 92)', () => {
  it('links the notifications inbox', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText('NOTIFICATIONS').closest('a')).toHaveAttribute('href', '/notifications');
  });
});
