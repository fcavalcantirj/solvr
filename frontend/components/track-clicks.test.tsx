import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

// One click listener for the whole site (SPEC.md 27.7). A click on, or inside, an element
// marked data-track="nav" or data-track="cta" sends nav_click or cta_click with the item
// and where it sits. Marking a link is one spread: {...trackNav('rooms', 'header')}.

const track = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track }));

import { TrackClicks } from './track-clicks';
import { TRACK_LOCATIONS, trackCta, trackNav } from '@/lib/track-attrs';

describe('trackNav and trackCta', () => {
  it('build the three attributes the listener reads', () => {
    expect(trackNav('rooms', 'header')).toEqual({
      'data-track': 'nav',
      'data-track-item': 'rooms',
      'data-track-location': 'header',
    });
    expect(trackCta('connect_agents', 'hero')).toEqual({
      'data-track': 'cta',
      'data-track-item': 'connect_agents',
      'data-track-location': 'hero',
    });
  });

  it('know the seven places a marked element can sit', () => {
    expect([...TRACK_LOCATIONS]).toEqual(['header', 'mobile_menu', 'docs_menu', 'account_menu', 'footer', 'hero', 'page']);
    // @ts-expect-error a place outside the list is a type error
    trackNav('rooms', 'sidebar');
  });
});

describe('TrackClicks', () => {
  beforeEach(() => {
    track.mockClear();
  });

  it('renders nothing', () => {
    const { container } = render(<TrackClicks />);
    expect(container).toBeEmptyDOMElement();
  });

  it('sends nav_click for a marked link, with its item and location', () => {
    render(
      <>
        <TrackClicks />
        <a href="#rooms" {...trackNav('rooms', 'header')}>
          ROOMS
        </a>
      </>,
    );
    fireEvent.click(screen.getByText('ROOMS'));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('nav_click', { item: 'rooms', location: 'header' });
  });

  it('sends cta_click for a marked button', () => {
    render(
      <>
        <TrackClicks />
        <button type="button" {...trackCta('connect_agents', 'hero')}>
          Connect agents now
        </button>
      </>,
    );
    fireEvent.click(screen.getByRole('button'));
    expect(track).toHaveBeenCalledWith('cta_click', { item: 'connect_agents', location: 'hero' });
  });

  it('counts a click that lands inside the marked element, on its icon or its label', () => {
    render(
      <>
        <TrackClicks />
        <a href="#docs" {...trackNav('api_docs', 'docs_menu')}>
          <span>
            API <svg data-testid="icon" />
          </span>
        </a>
      </>,
    );
    fireEvent.click(screen.getByTestId('icon'));
    expect(track).toHaveBeenCalledWith('nav_click', { item: 'api_docs', location: 'docs_menu' });
  });

  it('takes the nearest marked element when one sits inside another', () => {
    render(
      <>
        <TrackClicks />
        <div {...trackCta('card', 'page')}>
          <a href="#x" {...trackNav('inner', 'footer')}>
            inner
          </a>
        </div>
      </>,
    );
    fireEvent.click(screen.getByText('inner'));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('nav_click', { item: 'inner', location: 'footer' });
  });

  it('sends nothing for an unmarked element, or for a mark it does not know', () => {
    render(
      <>
        <TrackClicks />
        <a href="#plain">plain</a>
        <a href="#odd" data-track="banner" data-track-item="x" data-track-location="page">
          odd
        </a>
      </>,
    );
    fireEvent.click(screen.getByText('plain'));
    fireEvent.click(screen.getByText('odd'));
    fireEvent.click(document.body);
    expect(track).not.toHaveBeenCalled();
  });

  it('sees a click even when the element stops it from spreading', () => {
    render(
      <>
        <TrackClicks />
        <button type="button" onClick={(event) => event.stopPropagation()} {...trackCta('copy_prompt', 'page')}>
          Copy prompt
        </button>
      </>,
    );
    fireEvent.click(screen.getByRole('button'));
    expect(track).toHaveBeenCalledWith('cta_click', { item: 'copy_prompt', location: 'page' });
  });

  it('is one listener: two mounts would count twice, so the layout mounts it once', () => {
    const layout = readFileSync(join(__dirname, '..', 'app', 'layout.tsx'), 'utf8');
    expect(layout.match(/<TrackClicks\b/g)).toHaveLength(1);
  });

  it('stops listening when it is unmounted', () => {
    const { unmount } = render(
      <>
        <TrackClicks />
        <a href="#rooms" {...trackNav('rooms', 'header')}>
          ROOMS
        </a>
      </>,
    );
    unmount();
    document.body.innerHTML = '<a id="late" href="#x" data-track="nav" data-track-item="late" data-track-location="page">late</a>';
    fireEvent.click(document.getElementById('late') as HTMLElement);
    expect(track).not.toHaveBeenCalled();
    document.body.innerHTML = '';
  });
});
