import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SearchMethodBadge } from './search-method-badge';

// Mock tooltip to make it testable in jsdom
vi.mock('@/components/ui/tooltip', () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children, ...props }: { children: React.ReactNode; asChild?: boolean }) => <>{children}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="tooltip-content">{children}</div>
  ),
}));

describe('SearchMethodBadge', () => {
  it('renders semantic search badge when method is hybrid', () => {
    render(<SearchMethodBadge method="hybrid" />);

    expect(screen.getByText('Semantic search enabled')).toBeInTheDocument();
  });

  it('does not render anything when method is fulltext', () => {
    const { container } = render(<SearchMethodBadge method="fulltext" />);

    expect(container.firstChild).toBeNull();
  });

  it('does not render anything when method is undefined', () => {
    const { container } = render(<SearchMethodBadge method={undefined} />);

    expect(container.firstChild).toBeNull();
  });

  it('includes tooltip with explanation text', () => {
    render(<SearchMethodBadge method="hybrid" />);

    expect(
      screen.getByText('Using AI embeddings to find semantically similar content')
    ).toBeInTheDocument();
  });

  it('renders with data-testid for identification', () => {
    render(<SearchMethodBadge method="hybrid" />);

    expect(screen.getByTestId('search-method-badge')).toBeInTheDocument();
  });

  it('is styled as a subtle text-xs indicator', () => {
    render(<SearchMethodBadge method="hybrid" />);

    const badge = screen.getByTestId('search-method-badge');
    expect(badge.className).toContain('text-xs');
    expect(badge.className).toContain('text-muted-foreground');
  });
});
