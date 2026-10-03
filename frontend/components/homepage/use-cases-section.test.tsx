import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';

import { UseCasesSection } from './use-cases-section';
import { USE_CASES } from '@/lib/docs/use-cases';
import { WORKFLOW_GUIDES } from '@/lib/docs/workflow-guides';

// Right under the hero: what a visitor can do with two agents in a room. Each
// card says who does what, gives an instruction to paste, starts /connect with
// the matching preset already chosen, and links the guide that was tested.

const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();
const cards = () => screen.getAllByTestId('use-case-card');

describe('UseCasesSection', () => {
  it('shows the three use cases in order: plan & execute, share context, build & review', () => {
    render(<UseCasesSection />);
    expect(cards().map((card) => squish(within(card).getByRole('heading', { level: 3 }).textContent))).toEqual([
      'Plan & execute',
      'Share context',
      'Build & review',
    ]);
  });

  it('starts /connect with each card\'s preset already chosen', () => {
    render(<UseCasesSection />);
    const presets = cards().map((card) =>
      within(card).getByRole('link', { name: /connect/i }).getAttribute('href'),
    );
    expect(presets).toEqual([
      '/connect?preset=plan-and-build',
      '/connect?preset=collaborate',
      '/connect?preset=build-and-review',
    ]);
  });

  it('links each card to its guide', () => {
    render(<UseCasesSection />);
    const guides = cards().map((card) =>
      within(card).getByRole('link', { name: /read the guide/i }).getAttribute('href'),
    );
    expect(guides).toEqual([
      '/docs/guides/connect-planner-executor',
      '/docs/guides/share-context-between-agents',
      '/docs/guides/connect-builder-reviewer',
    ]);
  });

  it('gives each card an instruction to paste and says who does what', () => {
    render(<UseCasesSection />);
    cards().forEach((card, i) => {
      expect(within(card).getByTestId('use-case-example')).toHaveTextContent(USE_CASES[i].example);
      expect(card).toHaveTextContent(USE_CASES[i].roles);
    });
  });

  it("teaches the owner's planner flow: room, executor prompt with the skill, doubts, orders, summary", () => {
    render(<UseCasesSection />);
    const plan = squish(within(cards()[0]).getByTestId('use-case-example').textContent);
    for (const step of [
      'public or private',
      'join it as the planner',
      'prompt for my executor',
      'Solvr skill',
      'join the room as the executor',
      'doubts',
      'follow your orders',
      'summary',
    ]) {
      expect(plan).toContain(step);
    }
    expect(squish(cards()[0].textContent)).toMatch(/paste that prompt into your executor/i);
  });

  it('tells one agent to ask and the other to teach when sharing context', () => {
    render(<UseCasesSection />);
    const share = squish(cards()[1].textContent);
    expect(share).toMatch(/one .*ask/i);
    expect(share).toMatch(/teach/i);
  });

  it('links only guides that exist, with the same preset and example as the guide', () => {
    for (const useCase of USE_CASES) {
      const guide = WORKFLOW_GUIDES.find((g) => g.slug === useCase.guideSlug);
      expect(guide, useCase.guideSlug).toBeDefined();
      expect(guide!.preset).toBe(useCase.preset);
      if (guide!.example) expect(guide!.example).toBe(useCase.example);
    }
  });
});
