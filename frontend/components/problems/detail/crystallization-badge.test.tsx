import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { CrystallizationBadge } from './crystallization-badge';

describe('CrystallizationBadge', () => {
  it('renders nothing when crystallizationCid is undefined', () => {
    const { container } = render(<CrystallizationBadge />);
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when crystallizationCid is null', () => {
    const { container } = render(
      <CrystallizationBadge crystallizationCid={null as unknown as undefined} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when crystallizationCid is empty string', () => {
    const { container } = render(<CrystallizationBadge crystallizationCid="" />);
    expect(container.firstChild).toBeNull();
  });

  it('renders crystallized badge when CID is present', () => {
    render(<CrystallizationBadge crystallizationCid="QmTest123" />);
    expect(screen.getByText('CRYSTALLIZED')).toBeInTheDocument();
  });

  it('renders as a link to IPFS gateway when CID is present', () => {
    render(<CrystallizationBadge crystallizationCid="QmTest123" />);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute(
      'href',
      'https://ipfs.io/ipfs/QmTest123'
    );
  });

  it('opens in new tab', () => {
    render(<CrystallizationBadge crystallizationCid="QmTest123" />);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('renders with CIDv1 (baf...) format', () => {
    render(
      <CrystallizationBadge crystallizationCid="bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3okuez5" />
    );
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute(
      'href',
      'https://ipfs.io/ipfs/bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3okuez5'
    );
  });

  it('uses compact variant for list display', () => {
    render(
      <CrystallizationBadge crystallizationCid="QmTest123" variant="compact" />
    );
    expect(screen.getByText('CRYSTALLIZED')).toBeInTheDocument();
    // Compact should not show CID details
    expect(screen.queryByText('QmTest123')).not.toBeInTheDocument();
  });

  it('uses detailed variant for detail page display', () => {
    render(
      <CrystallizationBadge crystallizationCid="QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG" variant="detailed" />
    );
    expect(screen.getByText('CRYSTALLIZED')).toBeInTheDocument();
    // Detailed should show truncated CID (first 6 + ... + last 4)
    expect(screen.getByText('QmYwAP...PbdG')).toBeInTheDocument();
  });

  it('shows crystallized date when provided in detailed variant', () => {
    render(
      <CrystallizationBadge
        crystallizationCid="QmTest123"
        crystallizedAt="2026-02-15T10:30:00Z"
        variant="detailed"
      />
    );
    expect(screen.getByText('CRYSTALLIZED')).toBeInTheDocument();
  });
});
