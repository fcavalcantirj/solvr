import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import ConnectPage, { metadata } from './page';

// /connect stays a real, directly linkable page. It renders the SAME panel the
// index opens inline — same component, same GET /v1/connect contract — so the
// two surfaces cannot drift apart.

vi.mock('@/components/header', () => ({
  Header: () => <header data-testid="page-header" />,
}));

vi.mock('@/components/connect/connect-panel', () => ({
  ConnectPanel: ({ variant }: { variant?: string }) => (
    <div data-testid="page-connect-panel" data-variant={variant} />
  ),
}));

// The signed-in-only direct-create panel reads auth, so mock it here the same
// way the header is mocked — the page test never mounts an AuthProvider.
vi.mock('@/components/connect/direct-create-panel', () => ({
  DirectCreatePanel: () => <div data-testid="page-direct-create-panel" />,
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

describe('the /connect page', () => {
  it('renders the shared connection panel as the page', () => {
    render(<ConnectPage />);
    const panel = screen.getByTestId('page-connect-panel');
    expect(panel).toHaveAttribute('data-variant', 'page');
  });

  it('keeps the header above it and the footer below it', () => {
    render(<ConnectPage />);
    const header = screen.getByTestId('page-header');
    const panel = screen.getByTestId('page-connect-panel');
    const footer = screen.getByRole('navigation', { name: /footer/i });
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(header.compareDocumentPosition(panel) & 4).toBeTruthy();
    expect(panel.compareDocumentPosition(footer) & 4).toBeTruthy();
  });

  it('offers the signed-in direct-create path below the prompt-first panel', () => {
    render(<ConnectPage />);
    const panel = screen.getByTestId('page-connect-panel');
    const directCreate = screen.getByTestId('page-direct-create-panel');
    // DOCUMENT_POSITION_FOLLOWING = 4 — the prompt-first panel comes first.
    expect(panel.compareDocumentPosition(directCreate) & 4).toBeTruthy();
  });

  it('is its own canonical URL, so it can be linked and shared directly', () => {
    expect(metadata.alternates?.canonical).toBe('/connect');
    expect(metadata.title).toBeTruthy();
  });

  it('reads no authentication state and sends nobody to a login', () => {
    const source = read('app/connect/page.tsx');
    expect(source).not.toContain('use-auth');
    expect(source).not.toContain('/login');
    expect(source).not.toContain('router.push');
  });

  it('renders the same panel component the index opens inline', () => {
    const page = read('app/connect/page.tsx');
    const hero = read('components/hero-section.tsx');
    for (const source of [page, hero]) {
      expect(source).toContain('@/components/connect/connect-panel');
    }
  });
});
