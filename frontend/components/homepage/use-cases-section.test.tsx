import { render, screen, within } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { UseCasesSection } from './use-cases-section';
import { CONNECT_EXAMPLES } from '@/components/connect/connect-fixture';

// The home use cases (v1.3.5): the API's three example sentences in the compact Prompt,
// each copyable, opening /connect with its use case chosen and linking its guide.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();
const cards = () => screen.getAllByTestId('use-case-card');

describe('UseCasesSection', () => {
  it('shows the three use cases in order: plan & execute, share context, build & review', () => {
    render(<UseCasesSection examples={CONNECT_EXAMPLES} />);
    expect(cards().map((card) => squish(within(card).getByRole('heading', { level: 3 }).textContent))).toEqual([
      'Plan & execute',
      'Share context',
      'Build & review',
    ]);
  });

  it('shows each example sentence exactly as the API served it', () => {
    render(<UseCasesSection examples={CONNECT_EXAMPLES} />);
    cards().forEach((card, i) => {
      expect(within(card).getByTestId('prompt-sentence').textContent).toBe(CONNECT_EXAMPLES[i].prompt.text);
      expect(within(card).getByRole('button', { name: /copy prompt/i })).toBeInTheDocument();
    });
  });

  it("starts /connect with each card's use case already chosen, and links its guide", () => {
    render(<UseCasesSection examples={CONNECT_EXAMPLES} />);
    const links = cards().map((card) => within(card).getAllByRole('link').map((a) => a.getAttribute('href')));
    expect(links.map((l) => l.find((h) => h?.startsWith('/connect')))).toEqual([
      '/connect?preset=plan-and-build',
      '/connect?preset=collaborate',
      '/connect?preset=build-and-review',
    ]);
    expect(links.map((l) => l.find((h) => h?.startsWith('/docs/guides/')))).toEqual([
      '/docs/guides/connect-planner-executor',
      '/docs/guides/share-context-between-agents',
      '/docs/guides/connect-builder-reviewer',
    ]);
  });

  it('shows no endpoint and no step list', () => {
    const { container } = render(<UseCasesSection examples={CONNECT_EXAMPLES} />);
    const text = container.textContent ?? '';
    for (const forbidden of ['api.solvr.dev', '/v1/', 'curl']) expect(text).not.toContain(forbidden);
    expect(container.querySelector('ol, pre')).toBeNull();
  });

  it('keeps a way onward when the sentences could not be read', () => {
    render(<UseCasesSection examples={null} />);
    expect(screen.queryAllByTestId('use-case-card')).toHaveLength(0);
    expect(screen.getByRole('link', { name: /copy the sentence at connect/i })).toHaveAttribute('href', '/connect');
  });

  it('writes no sentence of its own', () => {
    const source = readFileSync(join(process.cwd(), 'components/homepage/use-cases-section.tsx'), 'utf8');
    for (const forbidden of ['Learn Solvr', 'skill.md', 'PLANNER', 'join it as the']) {
      expect(source).not.toContain(forbidden);
    }
  });
});
