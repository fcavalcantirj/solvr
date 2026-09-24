import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { RecentlyViewedRooms } from './recently-viewed-rooms';
import { api } from '@/lib/api';
import { recordRoomView, getRecentRooms } from '@/lib/recently-viewed-rooms';

vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: {
    fetchRoom: vi.fn(),
  },
}));

describe('RecentlyViewedRooms', () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.mocked(api.fetchRoom).mockReset();
    vi.mocked(api.fetchRoom).mockResolvedValue({} as never);
  });

  it('renders nothing when there are no recently viewed rooms', () => {
    const { container } = render(<RecentlyViewedRooms />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders a link for each recently viewed room, newest first', () => {
    recordRoomView({ slug: 'alpha', displayName: 'Alpha Room' });
    recordRoomView({ slug: 'beta', displayName: 'Beta Room' });
    render(<RecentlyViewedRooms />);
    expect(screen.getByText('Beta Room').closest('a')).toHaveAttribute('href', '/rooms/beta');
    expect(screen.getByText('Alpha Room').closest('a')).toHaveAttribute('href', '/rooms/alpha');
    const links = screen.getAllByRole('link');
    expect(links[0]).toHaveTextContent('Beta Room');
  });

  it('clears the list when the Clear control is used', () => {
    recordRoomView({ slug: 'alpha', displayName: 'Alpha Room' });
    render(<RecentlyViewedRooms />);
    fireEvent.click(screen.getByRole('button', { name: /clear/i }));
    expect(screen.queryByText('Alpha Room')).not.toBeInTheDocument();
    expect(getRecentRooms()).toEqual([]);
  });

  it('drops an entry the API reports as inaccessible and forgets it locally', async () => {
    recordRoomView({ slug: 'gone', displayName: 'Gone Room' });
    recordRoomView({ slug: 'ok', displayName: 'Ok Room' });
    vi.mocked(api.fetchRoom).mockImplementation((slug: string) =>
      slug === 'gone' ? Promise.reject(new Error('404')) : Promise.resolve({} as never),
    );
    render(<RecentlyViewedRooms />);
    await waitFor(() => {
      expect(screen.queryByText('Gone Room')).not.toBeInTheDocument();
    });
    expect(screen.getByText('Ok Room')).toBeInTheDocument();
    expect(getRecentRooms().map((r) => r.slug)).toEqual(['ok']);
  });
});
