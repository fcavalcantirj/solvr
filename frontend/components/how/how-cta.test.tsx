import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { HowCta } from './how-cta';

// /problems is a retired route (it answers a redirect to /posts). The closing section
// of /how-it-works linked it, so every crawl of that page followed a redirect.
const RETIRED = /^\/(feed|problems|ideas|questions)(\/|$)/;

describe('HowCta', () => {
  it('sends the browse action to the posts collection, under its own name', () => {
    render(<HowCta />);
    expect(screen.getByRole('link', { name: /browse posts/i })).toHaveAttribute('href', '/posts');
  });

  it('links no retired route', () => {
    const { container } = render(<HowCta />);
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href') ?? '');
    expect(hrefs.length).toBeGreaterThan(2);
    expect(hrefs.filter((href) => RETIRED.test(href))).toEqual([]);
  });
});
