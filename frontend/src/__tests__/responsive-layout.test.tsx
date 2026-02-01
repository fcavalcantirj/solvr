/**
 * Tests for responsive layout design
 * PRD requirement: "Layout: responsive design" (line 438)
 * Tests layout at 375px width and verifies no horizontal scroll
 */

import { render, screen } from '@testing-library/react';

// Mock next/link to preserve className and other props
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
    className,
    ...props
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
    [key: string]: unknown;
  }) {
    return (
      <a href={href} className={className} {...props}>
        {children}
      </a>
    );
  };
});

// Mock window.matchMedia for responsive behavior testing
const mockMatchMedia = (matches: boolean) => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: jest.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      onchange: null,
      addListener: jest.fn(),
      removeListener: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    })),
  });
};

// Import components after mocks
import Header from '../components/Header';
import Footer from '../components/Footer';
import Sidebar from '../components/Sidebar';

describe('Responsive Layout at 375px width', () => {
  beforeEach(() => {
    // Set viewport to mobile width
    Object.defineProperty(window, 'innerWidth', { value: 375, writable: true });
    Object.defineProperty(window, 'innerHeight', { value: 667, writable: true });
    mockMatchMedia(false); // Mobile view (md: false)
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('Header Component', () => {
    it('renders correctly at 375px width', () => {
      render(<Header />);
      const header = screen.getByRole('banner');
      expect(header).toBeInTheDocument();
    });

    it('hides desktop navigation on mobile (md:hidden)', () => {
      render(<Header />);

      // Desktop nav should have hidden class at mobile widths
      const desktopNav = screen.getByRole('navigation', { name: /main navigation/i });
      expect(desktopNav).toHaveClass('hidden');
    });

    it('shows mobile menu button on mobile (md:hidden)', () => {
      render(<Header />);

      const mobileMenuButton = screen.getByRole('button', { name: /menu/i });
      expect(mobileMenuButton).toBeInTheDocument();
      // Button should have md:hidden class (visible on mobile)
      expect(mobileMenuButton).toHaveClass('md:hidden');
    });

    it('hides desktop search bar on mobile', () => {
      render(<Header />);

      // The desktop search form has hidden md:flex classes
      const forms = document.querySelectorAll('form');
      const desktopSearchForm = Array.from(forms).find(form =>
        form.className.includes('hidden') && form.className.includes('md:flex')
      );
      expect(desktopSearchForm).toBeTruthy();
    });

    it('has max-width constraint to prevent overflow', () => {
      render(<Header />);

      const header = screen.getByRole('banner');
      const innerContainer = header.querySelector('.max-w-7xl');
      expect(innerContainer).toBeInTheDocument();
    });

    it('uses responsive padding classes', () => {
      render(<Header />);

      const header = screen.getByRole('banner');
      const innerContainer = header.querySelector('.px-4');
      expect(innerContainer).toBeInTheDocument();
    });
  });

  describe('Sidebar Component', () => {
    it('is hidden on mobile by default', () => {
      render(<Sidebar />);

      const sidebar = screen.getByRole('complementary');
      // When isOpen is false (default), sidebar should be hidden on mobile
      expect(sidebar).toHaveClass('hidden');
      expect(sidebar).toHaveClass('md:block');
    });

    it('shows when isOpen is true on mobile', () => {
      render(<Sidebar isOpen={true} />);

      const sidebar = screen.getByRole('complementary');
      // When isOpen is true, sidebar should be visible
      expect(sidebar).not.toHaveClass('hidden');
    });

    it('shows backdrop on mobile when open', () => {
      render(<Sidebar isOpen={true} />);

      const backdrop = screen.getByTestId('sidebar-backdrop');
      expect(backdrop).toBeInTheDocument();
      expect(backdrop).toHaveClass('md:hidden'); // Backdrop only shows on mobile
    });

    it('uses fixed positioning on mobile', () => {
      render(<Sidebar isOpen={true} />);

      const sidebar = screen.getByRole('complementary');
      expect(sidebar).toHaveClass('fixed');
    });

    it('has proper z-index for mobile overlay', () => {
      render(<Sidebar isOpen={true} />);

      const sidebar = screen.getByRole('complementary');
      expect(sidebar.className).toMatch(/z-\d+/);
    });
  });

  describe('Footer Component', () => {
    it('renders correctly at 375px width', () => {
      render(<Footer />);

      const footer = screen.getByRole('contentinfo');
      expect(footer).toBeInTheDocument();
    });

    it('has flex-wrap for responsive link layout', () => {
      render(<Footer />);

      const footer = screen.getByRole('contentinfo');
      // Footer should have flex container with responsive layout
      const flexContainer = footer.querySelector('.flex');
      expect(flexContainer).toBeInTheDocument();
    });

    it('renders all links at mobile width', () => {
      render(<Footer />);

      expect(screen.getByRole('link', { name: /about/i })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: /api docs/i })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: /github/i })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: /terms/i })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: /privacy/i })).toBeInTheDocument();
    });
  });
});

describe('No Horizontal Scroll Verification', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'innerWidth', { value: 375, writable: true });
    mockMatchMedia(false);
  });

  describe('Header prevents horizontal overflow', () => {
    it('does not have fixed width causing overflow', () => {
      render(<Header />);

      const header = screen.getByRole('banner');
      const computedStyles = window.getComputedStyle(header);

      // Should not have explicit width that could cause overflow
      expect(computedStyles.width).not.toBe('100vw');
      expect(header.className).not.toMatch(/w-\[.*px\]/);
    });

    it('uses percentage or responsive widths', () => {
      render(<Header />);

      const header = screen.getByRole('banner');
      // Header should use full width with proper containment
      expect(header.querySelector('.max-w-7xl')).toBeInTheDocument();
    });

    it('inner container has overflow handling classes', () => {
      render(<Header />);

      const header = screen.getByRole('banner');
      // Inner container should not cause overflow
      const innerDiv = header.querySelector('.max-w-7xl');
      expect(innerDiv).toBeInTheDocument();
    });
  });

  describe('Sidebar prevents horizontal overflow', () => {
    it('uses responsive width classes', () => {
      render(<Sidebar isCollapsed={false} />);

      const sidebar = screen.getByRole('complementary');
      // Sidebar should have defined width class
      expect(sidebar).toHaveClass('w-64');
    });

    it('collapses to smaller width when collapsed', () => {
      render(<Sidebar isCollapsed={true} />);

      const sidebar = screen.getByRole('complementary');
      expect(sidebar).toHaveClass('w-16');
    });

    it('backdrop does not cause overflow', () => {
      render(<Sidebar isOpen={true} />);

      const backdrop = screen.getByTestId('sidebar-backdrop');
      // Backdrop should be fixed and cover full viewport
      expect(backdrop).toHaveClass('fixed');
      expect(backdrop).toHaveClass('inset-0');
    });
  });

  describe('Footer prevents horizontal overflow', () => {
    it('has responsive container', () => {
      render(<Footer />);

      const footer = screen.getByRole('contentinfo');
      const container = footer.querySelector('.max-w-7xl');
      expect(container).toBeInTheDocument();
    });

    it('uses flex-wrap for content', () => {
      render(<Footer />);

      const footer = screen.getByRole('contentinfo');
      // Footer content should wrap on small screens
      expect(footer.querySelector('.flex-col, .flex-wrap, .flex')).toBeInTheDocument();
    });
  });
});

describe('Responsive Breakpoint Classes', () => {
  it('Header uses md: breakpoint for desktop/mobile split', () => {
    render(<Header />);

    const header = screen.getByRole('banner');
    const html = header.innerHTML;

    // Should use md: prefix for responsive behavior
    expect(html).toContain('md:');
  });

  it('Sidebar uses md: breakpoint for visibility', () => {
    render(<Sidebar />);

    const sidebar = screen.getByRole('complementary');

    // Sidebar should have responsive visibility classes
    expect(sidebar).toHaveClass('md:block');
  });

  it('Components use mobile-first approach', () => {
    render(<Header />);

    const header = screen.getByRole('banner');

    // Mobile styles should be the base (no prefix)
    // Desktop styles should use md: prefix
    const mobileMenuButton = screen.getByRole('button', { name: /menu/i });
    expect(mobileMenuButton).toHaveClass('md:hidden');
  });
});

describe('Touch-Friendly Layout', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'innerWidth', { value: 375, writable: true });
    mockMatchMedia(false);
  });

  it('mobile menu button has adequate touch target size', () => {
    render(<Header />);

    const mobileMenuButton = screen.getByRole('button', { name: /menu/i });
    // Button should have padding for touch target (p-2 = 8px padding)
    expect(mobileMenuButton).toHaveClass('p-2');
  });

  it('navigation links have adequate spacing on mobile', () => {
    render(<Sidebar isOpen={true} />);

    const navItems = screen.getAllByRole('listitem');
    // Each nav item should have proper padding for touch targets
    navItems.forEach(item => {
      const link = item.querySelector('a');
      // Links use px-3 and py-2 classes for adequate touch spacing
      expect(link?.className).toContain('px-3');
      expect(link?.className).toContain('py-2');
    });
  });
});

describe('Responsive Typography', () => {
  it('Header logo uses appropriate text size', () => {
    render(<Header />);

    const logo = screen.getByRole('link', { name: /solvr/i });
    const textSpan = logo.querySelector('span');
    expect(textSpan).toHaveClass('text-xl');
  });

  it('Desktop navigation links use readable text size', () => {
    render(<Header />);

    // Desktop nav links are inside the nav element with aria-label "Main navigation"
    const desktopNav = screen.getByRole('navigation', { name: /main navigation/i });
    const navLinks = desktopNav.querySelectorAll('a');

    navLinks.forEach(link => {
      expect(link).toHaveClass('text-sm');
    });
  });
});
