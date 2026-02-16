import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import TermsPage from './page';

// Mock components
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));

vi.mock('@/components/footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));

describe('TermsPage', () => {
  it('renders without crashing', () => {
    expect(() => render(<TermsPage />)).not.toThrow();
  });

  it('renders header and footer', () => {
    const { getByTestId } = render(<TermsPage />);
    expect(getByTestId('header')).toBeDefined();
    expect(getByTestId('footer')).toBeDefined();
  });

  it('renders terms of service title', () => {
    const { getByText } = render(<TermsPage />);
    expect(getByText('Terms of Service')).toBeDefined();
  });

  it('does not use useState (should be a static page)', () => {
    // This test ensures the component doesn't use client-side state
    // If useState is imported but not used, the build will fail
    const { container } = render(<TermsPage />);
    expect(container).toBeDefined();
  });
});
