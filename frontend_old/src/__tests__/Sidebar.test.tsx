/**
 * Tests for Sidebar component
 * Tests per PRD requirement: Create Sidebar component with nav links, collapsible on mobile
 */

import { render, screen, fireEvent } from '@testing-library/react';

// Mock next/link
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) {
    return (
      <a href={href} className={className}>
        {children}
      </a>
    );
  };
});

// Import component after mocks
import Sidebar from '../components/Sidebar';

describe('Sidebar', () => {
  it('renders the aside element', () => {
    render(<Sidebar />);

    const sidebar = screen.getByRole('complementary');
    expect(sidebar).toBeInTheDocument();
  });

  it('renders navigation links', () => {
    render(<Sidebar />);

    const nav = screen.getByRole('navigation');
    expect(nav).toBeInTheDocument();
  });

  it('renders Feed link', () => {
    render(<Sidebar />);

    const feedLink = screen.getByRole('link', { name: /feed/i });
    expect(feedLink).toBeInTheDocument();
    expect(feedLink).toHaveAttribute('href', '/feed');
  });

  it('renders Problems link', () => {
    render(<Sidebar />);

    const problemsLink = screen.getByRole('link', { name: /problems/i });
    expect(problemsLink).toBeInTheDocument();
    expect(problemsLink).toHaveAttribute('href', '/problems');
  });

  it('renders Questions link', () => {
    render(<Sidebar />);

    const questionsLink = screen.getByRole('link', { name: /questions/i });
    expect(questionsLink).toBeInTheDocument();
    expect(questionsLink).toHaveAttribute('href', '/questions');
  });

  it('renders Ideas link', () => {
    render(<Sidebar />);

    const ideasLink = screen.getByRole('link', { name: /ideas/i });
    expect(ideasLink).toBeInTheDocument();
    expect(ideasLink).toHaveAttribute('href', '/ideas');
  });

  it('renders Agents link', () => {
    render(<Sidebar />);

    const agentsLink = screen.getByRole('link', { name: /agents/i });
    expect(agentsLink).toBeInTheDocument();
    expect(agentsLink).toHaveAttribute('href', '/agents');
  });

  it('supports controlled collapsed state', () => {
    const { rerender } = render(<Sidebar isCollapsed={false} />);

    const sidebar = screen.getByRole('complementary');
    expect(sidebar).not.toHaveClass('collapsed');

    rerender(<Sidebar isCollapsed={true} />);
    expect(sidebar).toHaveClass('collapsed');
  });

  it('renders collapse toggle button', () => {
    render(<Sidebar onToggleCollapse={() => {}} />);

    const toggleButton = screen.getByRole('button', { name: /toggle sidebar/i });
    expect(toggleButton).toBeInTheDocument();
  });

  it('calls onToggleCollapse when toggle button is clicked', () => {
    const onToggleCollapse = jest.fn();
    render(<Sidebar onToggleCollapse={onToggleCollapse} />);

    const toggleButton = screen.getByRole('button', { name: /toggle sidebar/i });
    fireEvent.click(toggleButton);

    expect(onToggleCollapse).toHaveBeenCalledTimes(1);
  });

  it('shows icons only when collapsed', () => {
    render(<Sidebar isCollapsed={true} />);

    // When collapsed, labels are hidden but icons remain
    // Check that links still have icons
    const links = screen.getAllByRole('link');
    expect(links.length).toBeGreaterThan(0);

    // First link should have an icon
    const firstLink = links[0];
    expect(firstLink.querySelector('svg')).toBeInTheDocument();
  });

  it('highlights active link based on currentPath prop', () => {
    render(<Sidebar currentPath="/problems" />);

    const problemsLink = screen.getByRole('link', { name: /problems/i });
    expect(problemsLink).toHaveClass('active');

    const feedLink = screen.getByRole('link', { name: /feed/i });
    expect(feedLink).not.toHaveClass('active');
  });

  it('has appropriate aria-label for navigation', () => {
    render(<Sidebar />);

    const nav = screen.getByRole('navigation');
    expect(nav).toHaveAttribute('aria-label');
  });

  it('shows Create section with quick actions', () => {
    render(<Sidebar />);

    // Should have create section
    expect(screen.getByText(/create/i)).toBeInTheDocument();

    // Should have create links
    expect(screen.getByRole('link', { name: /new problem/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /new question/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /new idea/i })).toBeInTheDocument();
  });

  it('renders correct hrefs for create links', () => {
    render(<Sidebar />);

    expect(screen.getByRole('link', { name: /new problem/i })).toHaveAttribute(
      'href',
      '/new/problem'
    );
    expect(screen.getByRole('link', { name: /new question/i })).toHaveAttribute(
      'href',
      '/new/question'
    );
    expect(screen.getByRole('link', { name: /new idea/i })).toHaveAttribute(
      'href',
      '/new/idea'
    );
  });
});

describe('Sidebar mobile behavior', () => {
  it('hides on mobile when not explicitly shown', () => {
    render(<Sidebar />);

    const sidebar = screen.getByRole('complementary');
    // Has class that hides on mobile
    expect(sidebar).toHaveClass('hidden');
    expect(sidebar).toHaveClass('md:block');
  });

  it('can be shown on mobile via isOpen prop', () => {
    render(<Sidebar isOpen={true} />);

    const sidebar = screen.getByRole('complementary');
    expect(sidebar).not.toHaveClass('hidden');
  });

  it('calls onClose when backdrop is clicked on mobile', () => {
    const onClose = jest.fn();
    render(<Sidebar isOpen={true} onClose={onClose} />);

    const backdrop = screen.getByTestId('sidebar-backdrop');
    fireEvent.click(backdrop);

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe('Sidebar styling', () => {
  it('has appropriate width classes', () => {
    render(<Sidebar />);

    const sidebar = screen.getByRole('complementary');
    expect(sidebar.className).toMatch(/w-\d+|w-\[\d+/);
  });

  it('has border styling', () => {
    render(<Sidebar />);

    const sidebar = screen.getByRole('complementary');
    expect(sidebar.className).toContain('border');
  });

  it('has appropriate height', () => {
    render(<Sidebar />);

    const sidebar = screen.getByRole('complementary');
    expect(sidebar.className).toMatch(/h-/);
  });
});
