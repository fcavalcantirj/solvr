import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { RoomArchiveNav, archivePageNumbers } from './room-archive-nav';

// Task idx 81: the room page links its transcript archive and its outcome posts with
// ordinary anchors, so a crawler without JavaScript can reach every earlier message.

describe('archivePageNumbers', () => {
  it('lists every page of a short archive', () => {
    expect(archivePageNumbers(3)).toEqual([1, 2, 3]);
    expect(archivePageNumbers(20)).toHaveLength(20);
  });

  it('lists both ends of a long archive; the pages chain the rest', () => {
    expect(archivePageNumbers(50)).toEqual([1, 2, 3, 48, 49, 50]);
  });

  it('lists nothing for an empty room', () => {
    expect(archivePageNumbers(0)).toEqual([]);
  });
});

describe('RoomArchiveNav', () => {
  it('links each transcript page and each outcome post', () => {
    const { container } = render(
      <RoomArchiveNav
        slug="kestrel"
        history={{ page_size: 100, total_pages: 2 }}
        outcomes={[{ id: 'p1', title: 'Kestrel firmware shipped' }]}
      />
    );
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/rooms/kestrel/history/1', '/rooms/kestrel/history/2', '/posts/p1']);
    expect(container.textContent).toContain('Kestrel firmware shipped');
  });

  it('renders nothing without an archive or outcomes', () => {
    const { container } = render(
      <RoomArchiveNav slug="kestrel" history={{ page_size: 100, total_pages: 0 }} outcomes={[]} />
    );
    expect(container.innerHTML).toBe('');
  });
});
