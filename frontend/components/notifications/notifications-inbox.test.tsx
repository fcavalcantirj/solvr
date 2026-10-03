import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('next/link', () => ({
  default: ({ children, href, onClick }: { children: React.ReactNode; href: string; onClick?: () => void }) => (
    <a href={href} onClick={onClick}>{children}</a>
  ),
}));
vi.mock('@/lib/api', () => ({
  api: {
    listNotifications: vi.fn(),
    markNotificationRead: vi.fn(),
    markAllNotificationsRead: vi.fn(),
    getNotificationSettings: vi.fn(),
    updateNotificationSettings: vi.fn(),
  },
  formatRelativeTime: () => '1h ago',
}));

import { api } from '@/lib/api';
import { NotificationsInbox } from './notifications-inbox';

// The human inbox (idx 92): the API's notifications, read state and the global switch.
const reply = {
  id: 'n-1', type: 'room.reply', title: 'New reply in Board', body: 'planner replied to you in "Board". Stop these: …',
  link: '/rooms/board?message=7', read_at: null, created_at: '2026-10-02T00:00:00Z', schema_version: 3,
  subject: { room_id: 'r-1', entry_id: 7 },
};

beforeEach(() => {
  vi.mocked(api.listNotifications).mockResolvedValue({ data: [reply], meta: { total: 1, page: 1, per_page: 50, has_more: false } });
  vi.mocked(api.getNotificationSettings).mockResolvedValue({ data: { room_notifications: 'on' } });
  vi.mocked(api.markNotificationRead).mockResolvedValue(undefined);
  vi.mocked(api.markAllNotificationsRead).mockResolvedValue(undefined);
  vi.mocked(api.updateNotificationSettings).mockResolvedValue({ data: { room_notifications: 'paused' } });
});

describe('NotificationsInbox', () => {
  it('lists notifications with their link', async () => {
    render(<NotificationsInbox />);
    expect(await screen.findByText('New reply in Board')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /open/i })).toHaveAttribute('href', '/rooms/board?message=7');
  });

  it('marks one read when opened, and all read on request', async () => {
    render(<NotificationsInbox />);
    fireEvent.click(await screen.findByRole('link', { name: /open/i }));
    await waitFor(() => expect(api.markNotificationRead).toHaveBeenCalledWith('n-1'));
    fireEvent.click(screen.getByRole('button', { name: /mark all read/i }));
    await waitFor(() => expect(api.markAllNotificationsRead).toHaveBeenCalled());
  });

  it('pauses every room notification and shows the answered state', async () => {
    render(<NotificationsInbox />);
    fireEvent.click(await screen.findByRole('button', { name: /pause room notifications/i }));
    await waitFor(() => expect(api.updateNotificationSettings).toHaveBeenCalledWith('paused'));
    expect(await screen.findByRole('button', { name: /resume room notifications/i })).toBeInTheDocument();
  });

  it('says when there is nothing yet', async () => {
    vi.mocked(api.listNotifications).mockResolvedValue({ data: [], meta: { total: 0, page: 1, per_page: 50, has_more: false } });
    render(<NotificationsInbox />);
    expect(await screen.findByText(/no notifications/i)).toBeInTheDocument();
  });
});
