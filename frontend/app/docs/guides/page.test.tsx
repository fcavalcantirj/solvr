import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import GuidesPage from './page';
import { WORKFLOW_GUIDES } from '@/lib/docs/workflow-guides';

// /docs/guides, rooms-era (v1.3.4): one paragraph on what a room is, the tested
// use-case guides as cards, how a room works in four steps with one curl example,
// and where to go next. The old knowledge-base page (search-first pattern, the
// OpenClaw auth gotcha, the guide carousel) is gone.

// Mock Header and Footer components
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));

vi.mock('@/components/footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();
const hrefs = (container: HTMLElement) =>
  [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));

describe('GuidesPage', () => {
  it('should not render "MORE GUIDES COMING SOON" placeholder text', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('MORE GUIDES COMING SOON')).not.toBeInTheDocument();
  });

  it('should NOT render "Contributing Solutions" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Contributing Solutions')).not.toBeInTheDocument();
  });

  it('should NOT render "Authentication Flows" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Authentication Flows')).not.toBeInTheDocument();
  });

  it('should NOT render "MCP Server Integration" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('MCP Server Integration')).not.toBeInTheDocument();
  });

  it('should NOT render "Rate Limits & Best Practices" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Rate Limits & Best Practices')).not.toBeInTheDocument();
  });

  it('should NOT render "Documentation is evolving" placeholder heading', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Documentation is evolving')).not.toBeInTheDocument();
  });

  // Replaces 'should render page heading "Build with Solvr"'.
  it('says in one paragraph what a room is: agent to agent, any client, nothing to install', () => {
    render(<GuidesPage />);
    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
    const intro = squish(screen.getByTestId('guides-intro').textContent);
    expect(intro).toMatch(/agents? talk to each other|agent-to-agent/i);
    expect(intro).toMatch(/any agent|any client/i);
    expect(intro).toMatch(/nothing to install|no install/i);
  });

  // Replaces 'should render 4 unique guide cards (duplicated for carousel)',
  // 'should render "ALL GUIDES" section heading' and the card/link tests for the
  // old guides ('Getting Started with AI Agents', 'Search Before You Solve',
  // 'Give Before You Take', 'OpenClaw').
  it('lists the use-case guides as cards, in order, each linking its guide page', () => {
    render(<GuidesPage />);
    const cards = screen.getAllByTestId('guide-card');
    expect(cards.map((card) => squish(within(card).getByRole('heading', { level: 3 }).textContent))).toEqual(
      WORKFLOW_GUIDES.map((guide) => guide.title),
    );
    expect(cards.map((card) => within(card).getByRole('link').getAttribute('href'))).toEqual(
      WORKFLOW_GUIDES.map((guide) => `/docs/guides/${guide.slug}`),
    );
    expect(WORKFLOW_GUIDES.map((guide) => guide.slug)).toEqual([
      'connect-planner-executor',
      'share-context-between-agents',
      'connect-builder-reviewer',
      'resume-across-two-clis',
    ]);
  });

  // Replaces 'should render guide content sections' (the 00–03 knowledge-base sections).
  it('explains how a room works in four steps: register, room, handshake, entries', () => {
    render(<GuidesPage />);
    const section = screen.getByTestId('how-a-room-works');
    const steps = within(section)
      .getAllByRole('listitem')
      .map((li) => squish(li.textContent));
    expect(steps).toHaveLength(4);
    expect(steps[0]).toMatch(/^Register/);
    expect(steps[1]).toMatch(/^Room/);
    expect(steps[2]).toMatch(/^Handshake/);
    expect(steps[3]).toMatch(/^Entries/);
  });

  // Replaces 'should use prompt-first content instead of curl commands', which asserted
  // that no code block contained curl. The owner-approved rework asks for ONE curl
  // example under How a room works; the prompts stay the way to start.
  it('shows exactly one curl example, under How a room works, with the calls the prompts teach', () => {
    const { container } = render(<GuidesPage />);
    const curlBlocks = [...container.querySelectorAll('code')].filter((code) =>
      code.textContent?.includes('curl'),
    );
    expect(curlBlocks).toHaveLength(1);
    expect(screen.getByTestId('how-a-room-works')).toContainElement(curlBlocks[0]);
    const example = curlBlocks[0].textContent ?? '';
    for (const call of [
      'https://api.solvr.dev/v1/agents/register',
      'https://api.solvr.dev/v1/rooms',
      '/handshake',
      'https://api.solvr.dev/r/',
      '/entries',
    ]) {
      expect(example).toContain(call);
    }
  });

  // Replaces 'should render Solvr skill install reference': the skill is linked, not installed.
  it('links where to go next: /connect, the skill, llms.txt and the API docs', () => {
    const { container } = render(<GuidesPage />);
    const links = hrefs(container);
    for (const href of ['/connect', '/skill.md', '/llms.txt', '/api-docs']) {
      expect(links).toContain(href);
    }
  });

  // Replaces 'should render "Search Before You Solve" guide card', 'should render OpenClaw
  // guide card', 'should have clickable link for OpenClaw guide', 'should render OpenClaw
  // section content', 'should render the 4-layer auth override example prompt' and
  // 'should link to Solvr post for full 4-layer reference'.
  it('drops the carousel, the OpenClaw section and the search-first pattern', () => {
    const { container } = render(<GuidesPage />);
    const text = container.textContent ?? '';
    expect(text).not.toMatch(/openclaw/i);
    expect(text).not.toContain('Search Before You Solve');
    expect(text).not.toContain('Give Before You Take');
    expect(text).not.toContain('ONLY START DOING WORK AFTER FINDING THE POST');
    expect(text).not.toContain('ALL GUIDES');
    expect(container.querySelector('.animate-carousel')).toBeNull();
    expect(hrefs(container)).not.toContain('https://solvr.dev/ideas/44781b98-68f2-4ffb-9ad0-cbec604393a4');
  });

  it('is a server-rendered page: no client-only code', () => {
    const source = readFileSync(join(process.cwd(), 'app/docs/guides/page.tsx'), 'utf8');
    expect(source).not.toMatch(/^["']use client["']/m);
  });
});
