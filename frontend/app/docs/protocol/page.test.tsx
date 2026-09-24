import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import ProtocolPage from './page';

// Header/Footer are client islands with their own state; stub them so this test
// exercises only the protocol documentation content.
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));
vi.mock('@/components/footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe('Docs > Protocol (task: describe actual A2A-related capabilities)', () => {
  // Step 4: keep "agent-to-agent" as plain language and put the protocol detail in Docs.
  it('renders an agent-to-agent capabilities heading', () => {
    render(<ProtocolPage />);
    expect(
      screen.getByRole('heading', { name: /agent-to-agent/i })
    ).toBeInTheDocument();
  });

  // Step 3: do not imply full conformance — say plainly what Solvr is NOT.
  it('states plainly that Solvr is not a conformant A2A protocol server', () => {
    render(<ProtocolPage />);
    expect(screen.getByText(/not a conformant A2A/i)).toBeInTheDocument();
  });

  // Step 1: record the official A2A spec version actually compared against.
  it('records the official A2A specification version it was compared against', () => {
    const { container } = render(<ProtocolPage />);
    expect(container.textContent).toContain('1.0.0');
    expect(container.textContent).toMatch(/A2A specification/i);
  });

  // Step 2: document the real room transport with executable examples.
  it('documents the real /r/{slug} room transport endpoints', () => {
    const { container } = render(<ProtocolPage />);
    expect(container.textContent).toContain('/r/{slug}/message');
    expect(container.textContent).toContain('/r/{slug}/stream');
    expect(container.textContent).toContain('/r/{slug}/join');
  });

  it('shows executable examples using the room bearer token', () => {
    const { container } = render(<ProtocolPage />);
    expect(container.textContent).toContain('curl');
    expect(container.textContent).toContain('Authorization: Bearer');
  });

  // Step 2: document the supported Agent Card fields and the public strip.
  it('documents the supported Agent Card fields and the public-listing strip', () => {
    const { container } = render(<ProtocolPage />);
    expect(container.textContent).toMatch(/Agent Card/i);
    expect(container.textContent).toContain('securitySchemes');
  });

  // Step 3: list the standard A2A operations Solvr does NOT support.
  it('lists unsupported standard A2A operations instead of implying conformance', () => {
    const { container } = render(<ProtocolPage />);
    expect(container.textContent).toContain('message/send');
    expect(container.textContent).toContain('tasks/get');
    expect(container.textContent).toMatch(/JSON-RPC/i);
  });

  // Step 5: the default connection path needs only ordinary outbound HTTP.
  it('states the connection path requires only outbound HTTP capabilities', () => {
    render(<ProtocolPage />);
    expect(screen.getByText(/only outbound HTTP/i)).toBeInTheDocument();
    expect(screen.getByText(/No A2A SDK/i)).toBeInTheDocument();
  });
});
