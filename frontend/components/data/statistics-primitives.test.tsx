import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MetricNote, StatisticsDetails, StatisticsHeading } from './statistics-primitives';

// The pieces /data is built from (v1.3.7 look): a section opens on its heading, a
// figure's window opens its fine print, and a disclosure is named by an API string.

describe('StatisticsHeading', () => {
  it('opens on the heading itself, with nothing set above it', () => {
    const { container } = render(<StatisticsHeading heading="Rooms, live" intro="Agents work in rooms." />);
    const heading = screen.getByRole('heading', { level: 2, name: 'Rooms, live' });
    expect(heading.previousElementSibling).toBeNull();
    expect(container.firstElementChild?.firstElementChild).toBe(heading);
    expect(screen.getByText('Agents work in rooms.')).toBeInTheDocument();
  });

  it('carries what belongs beside the heading after its intro', () => {
    render(
      <StatisticsHeading heading="What agents call" intro="Measured at the boundary.">
        <a href="/api-docs">Read the API reference</a>
      </StatisticsHeading>,
    );
    expect(screen.getByRole('link', { name: 'Read the API reference' })).toHaveAttribute('href', '/api-docs');
  });
});

describe('MetricNote', () => {
  it('names the window and keeps the definition and caveat behind it', () => {
    const { container } = render(
      <MetricNote window="last 24 hours" definition="What the number counts." qualifier="Measured since 4 Oct." />,
    );
    const details = container.querySelector('details');
    expect(details).not.toBeNull();
    expect(details).not.toHaveAttribute('open');
    expect(within(details!.querySelector('summary')!).getByText('last 24 hours')).toBeInTheDocument();
    expect(screen.getByText('What the number counts.')).toBeInTheDocument();
    expect(screen.getByText('Measured since 4 Oct.')).toBeInTheDocument();
  });

  it('adds no caveat line when the API sent none', () => {
    const { container } = render(<MetricNote window="all time" definition="Every room ever opened." />);
    expect(container.querySelectorAll('details p')).toHaveLength(1);
  });
});

describe('StatisticsDetails', () => {
  it('is named by the label it was given and starts closed', () => {
    const { container } = render(
      <StatisticsDetails label="SEARCHES PER HOUR">
        <p>rows</p>
      </StatisticsDetails>,
    );
    expect(within(container.querySelector('summary')!).getByText('SEARCHES PER HOUR')).toBeInTheDocument();
    expect(container.querySelector('details')).not.toHaveAttribute('open');
  });

  it('opens by default when the list is what the reader came for', () => {
    const { container } = render(
      <StatisticsDetails label="TOP SEARCHES" defaultOpen>
        <p>rows</p>
      </StatisticsDetails>,
    );
    expect(container.querySelector('details')).toHaveAttribute('open');
  });
});
