import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';

// The calls to action of the home page, /connect, /how-it-works, /api-docs, /skill, /mcp and
// the guides are marked for the site's one click listener (SPEC.md 27.7). item says WHAT
// was pressed, as a stable id that survives a change of wording; location says WHERE: in
// the page's hero or further down the page. This file is the list.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('@/lib/api', () => ({
  api: { getConnectStart: vi.fn(), postFunnelEvent: vi.fn() },
}));

import { api } from '@/lib/api';
import { CONNECT_EXAMPLES, CONNECT_START } from '@/components/connect/connect-fixture';
import { OVERVIEW } from '@/components/homepage/overview-fixture';
import { LISTED_GUIDES } from '@/lib/docs/workflow-guides';
import { TRACK_LOCATIONS } from '@/lib/track-attrs';

import { HeroSection } from '@/components/hero-section';
import { ClosingSection } from '@/components/homepage/closing-section';
import { UseCasesSection } from '@/components/homepage/use-cases-section';
import { ConnectPanel } from '@/components/connect/connect-panel';
import { GuidePrompt } from '@/components/prompt/guide-prompt';
import { HowCta } from '@/components/how/how-cta';
import { ApiCta } from '@/components/api/api-cta';
import { ApiQuickstart } from '@/components/api/api-quickstart';
import { SkillHero } from '@/components/skill/skill-hero';
import { SkillInstall } from '@/components/skill/skill-install';
import { SkillPreview } from '@/components/skill/skill-preview';
import { McpHero } from '@/components/mcp/mcp-hero';
import { McpSetup } from '@/components/mcp/mcp-setup';
import GuidesPage from '@/app/docs/guides/page';
import GuidePage from '@/app/docs/guides/[slug]/page';

/** Every marked call to action under root, as [item, location, what it is]. */
function cta(root: ParentNode = document.body): string[][] {
  return Array.from(root.querySelectorAll('[data-track="cta"]')).map((el) => [
    el.getAttribute('data-track-item') ?? '',
    el.getAttribute('data-track-location') ?? '',
    el.tagName === 'A' ? (el.getAttribute('href') ?? '') : el.tagName.toLowerCase(),
  ]);
}

const fetchMock = vi.fn();

beforeEach(() => {
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START });
  vi.mocked(api.postFunnelEvent).mockReset();
  vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url: string) => ({
    ok: true,
    status: 200,
    json: async () =>
      String(url).includes('/v1/connect/examples')
        ? { data: { instruction_version: '2.0', presets: CONNECT_EXAMPLES } }
        : { data: { prompt: { text: 'Learn Solvr from https://solvr.dev/skill.md. SEEDED SENTENCE' } } },
  }));
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('home page', () => {
  it('marks the two hero actions', () => {
    render(<HeroSection />);
    expect(cta()).toEqual([
      ['connect_agents', 'hero', 'button'],
      ['watch_example', 'hero', '#example'],
    ]);
  });

  it('marks the closing action', () => {
    render(<ClosingSection data={OVERVIEW.closing} />);
    expect(cta()).toEqual([['connect_agents', 'page', '/connect']]);
  });

  it('marks the three actions of every use-case card', () => {
    render(<UseCasesSection examples={CONNECT_EXAMPLES} />);
    const cards = screen.getAllByTestId('use-case-card');
    expect(cards).toHaveLength(3);
    cards.forEach((card, i) => {
      const guide = LISTED_GUIDES.find((g) => g.preset === CONNECT_EXAMPLES[i].value)!;
      expect(cta(card)).toEqual([
        ['copy_prompt', 'page', 'button'],
        ['make_it_yours', 'page', `/connect?preset=${CONNECT_EXAMPLES[i].value}`],
        ['guide', 'page', `/docs/guides/${guide.slug}`],
      ]);
    });
  });

  it('marks the way onward when the sentences could not be read', () => {
    render(<UseCasesSection examples={null} />);
    expect(cta()).toEqual([['connect_agents', 'page', '/connect']]);
  });
});

describe('/connect and the panel the hero opens', () => {
  it('marks Copy prompt and the example link on the page', async () => {
    render(<ConnectPanel variant="page" />);
    await screen.findByTestId('connect-panel');
    expect(cta()).toEqual([
      ['copy_prompt', 'page', 'button'],
      ['example', 'page', CONNECT_START.example.url],
    ]);
  });

  it('marks the same two as hero actions inside the home page', async () => {
    render(<ConnectPanel variant="panel" />);
    await screen.findByTestId('connect-panel');
    expect(cta().map(([item, location]) => [item, location])).toEqual([
      ['copy_prompt', 'hero'],
      ['example', 'hero'],
    ]);
  });

  it('puts the mark on the button that is pressed, not around it', async () => {
    render(<ConnectPanel variant="page" />);
    await screen.findByTestId('connect-panel');
    expect(screen.getByRole('button', { name: /copy prompt/i })).toHaveAttribute('data-track', 'cta');
  });
});

describe('guides', () => {
  it('marks every guide card and every way onward on the index', async () => {
    render(await GuidesPage());
    expect(cta()).toEqual([
      ['connect_planner_executor', 'page', '/docs/guides/connect-planner-executor'],
      ['share_context_between_agents', 'page', '/docs/guides/share-context-between-agents'],
      ['connect_builder_reviewer', 'page', '/docs/guides/connect-builder-reviewer'],
      ['connect_agents', 'page', '/connect'],
      ['skill_md', 'page', '/skill.md'],
      ['llms_txt', 'page', '/llms.txt'],
      ['api_docs', 'page', '/api-docs'],
    ]);
  });

  it("marks a guide's Copy prompt", () => {
    render(<GuidePrompt example={CONNECT_EXAMPLES[0]} />);
    expect(cta()).toEqual([['copy_prompt', 'page', 'button']]);
  });

  it('marks Copy prompt on a use-case guide page', async () => {
    render(await GuidePage({ params: Promise.resolve({ slug: LISTED_GUIDES[0].slug }) }));
    expect(cta()).toEqual([['copy_prompt', 'page', 'button']]);
  });

  it('marks the way to Connect when a guide cannot show its sentence', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    render(await GuidePage({ params: Promise.resolve({ slug: LISTED_GUIDES[0].slug }) }));
    expect(cta()).toEqual([['connect_agents', 'page', `/connect?preset=${LISTED_GUIDES[0].preset}`]]);
  });

  it('marks the way to Connect on the resume guide', async () => {
    render(await GuidePage({ params: Promise.resolve({ slug: 'resume-across-two-clis' }) }));
    expect(cta()).toEqual([['connect_agents', 'page', '/connect']]);
  });
});

describe('/how-it-works, /api-docs, /skill and /mcp', () => {
  it('marks the closing actions of /how-it-works', () => {
    render(<HowCta />);
    expect(cta()).toEqual([
      ['read_docs', 'page', '/api-docs'],
      ['browse_posts', 'page', '/posts'],
      ['github', 'page', 'https://github.com/fcavalcantirj/solvr'],
    ]);
  });

  it('marks the first step and the closing band of /api-docs', () => {
    const { unmount } = render(<ApiQuickstart />);
    expect(cta()).toEqual([['get_api_key', 'page', '/join']]);
    unmount();

    render(<ApiCta />);
    expect(cta()).toEqual([
      ['get_api_key', 'page', '/settings/api-keys'],
      ['explore_posts', 'page', '/posts'],
      ['openapi_spec', 'page', 'https://api.solvr.dev/v1/openapi.json'],
      ['github', 'page', 'https://github.com/fcavalcantirj/solvr'],
      ['guides', 'page', '/docs/guides'],
    ]);
  });

  it('marks the download, the source and the claim link of /skill', () => {
    const { unmount } = render(<SkillHero />);
    expect(cta()).toEqual([
      ['download_zip', 'hero', '/solvr-skill.zip'],
      ['github', 'hero', 'https://github.com/fcavalcantirj/solvr/tree/main/skill'],
      ['claim_agent', 'page', '/settings/agents'],
    ]);
    unmount();

    const install = render(<SkillInstall />);
    expect(cta()).toEqual([
      ['download_zip', 'page', '/solvr-skill.zip'],
      ['github', 'page', 'https://github.com/fcavalcantirj/solvr/tree/main/skill'],
    ]);
    install.unmount();

    render(<SkillPreview />);
    expect(cta()).toEqual([['skill_md', 'page', '/skill.md']]);
  });

  it('marks the docs link and the key action of /mcp', () => {
    const { unmount } = render(<McpHero />);
    expect(cta()).toEqual([['api_docs', 'hero', '/api-docs']]);
    unmount();

    render(<McpSetup />);
    expect(cta()).toEqual([['get_api_key', 'page', '/settings/api-keys']]);
  });
});

describe('every mark, wherever it is', () => {
  it('names a known place and a stable lowercase item', async () => {
    render(
      <>
        <HeroSection />
        <ClosingSection data={OVERVIEW.closing} />
        <UseCasesSection examples={CONNECT_EXAMPLES} />
        <HowCta />
        <ApiQuickstart />
        <ApiCta />
        <SkillHero />
        <SkillInstall />
        <SkillPreview />
        <McpHero />
        <McpSetup />
        {await GuidesPage()}
      </>,
    );
    const marks = cta();
    expect(marks.length).toBeGreaterThan(30);
    for (const [item, location] of marks) {
      expect(item).toMatch(/^[a-z][a-z0-9_]*$/);
      expect(TRACK_LOCATIONS as readonly string[]).toContain(location);
    }
  });
});
