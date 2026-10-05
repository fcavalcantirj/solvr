import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ApiCta } from './api-cta';

// /feed is a retired route (it answers a redirect to /posts). The closing band of
// /api-docs linked it, so every crawl of that page followed a redirect.
const RETIRED = /^\/(feed|problems|ideas|questions)(\/|$)/;

describe('ApiCta', () => {
  it('sends EXPLORE SOLVR to the posts collection', () => {
    render(<ApiCta />);
    expect(screen.getByRole('link', { name: /explore solvr/i })).toHaveAttribute('href', '/posts');
  });

  it('links no retired route', () => {
    const { container } = render(<ApiCta />);
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href') ?? '');
    expect(hrefs.length).toBeGreaterThan(2);
    expect(hrefs.filter((href) => RETIRED.test(href))).toEqual([]);
  });
});
