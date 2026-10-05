import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

// code_copy (SPEC.md 27.7): every Copy button of the reference pages says which page family
// it sits on (surface) and WHAT was copied, as a stable id (item). It is sent once the
// clipboard took the text, and never carries the text: the API playground's curl command
// holds the token the visitor typed.

const track = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage: vi.fn() }));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

import { CopyButton } from '@/components/page/copy-button';
import { SkillHero } from '@/components/skill/skill-hero';
import { SkillInstall } from '@/components/skill/skill-install';
import { SkillPreview } from '@/components/skill/skill-preview';
import { McpHero } from '@/components/mcp/mcp-hero';
import { McpSetup } from '@/components/mcp/mcp-setup';
import { ApiHero } from '@/components/api/api-hero';
import { ApiQuickstart } from '@/components/api/api-quickstart';
import { ApiSdks } from '@/components/api/api-sdks';
import { ApiMcp } from '@/components/api/api-mcp';
import { ApiEndpoints } from '@/components/api/api-endpoints';
import { ApiPlayground } from '@/components/api/api-playground';
import { HowSolvr } from '@/components/how/how-solvr';
import { AmcpRecovery } from '@/components/amcp/amcp-recovery';
import { IpfsApi } from '@/components/ipfs/ipfs-api';

let writeText: ReturnType<typeof vi.fn>;
const copyButtons = () => screen.getAllByRole('button', { name: /^cop(y|ied)$/i });

/** Presses every Copy button now on the page, in order, and returns what each reported. */
async function pressEveryCopy(): Promise<Array<Record<string, unknown>>> {
  const buttons = copyButtons();
  for (const [i, button] of buttons.entries()) {
    fireEvent.click(button);
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(i + 1));
    await act(async () => {});
  }
  expect(track.mock.calls.every(([event]) => event === 'code_copy')).toBe(true);
  return track.mock.calls.map(([, params]) => params);
}

/** No report repeats a line of what was copied. */
function expectNoCopiedText() {
  const copied = writeText.mock.calls.map(([text]) => String(text));
  const sent = JSON.stringify(track.mock.calls);
  for (const text of copied) {
    for (const line of text.split('\n').map((l) => l.trim()).filter((l) => l.length > 12)) {
      expect(sent).not.toContain(line);
    }
  }
  expect(sent).not.toMatch(/curl|https?:|Bearer|solvr_|\$\{/);
}

beforeEach(() => {
  track.mockReset();
  writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({}) }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('CopyButton', () => {
  it('reports the copy once the clipboard took the text, with its surface and item', async () => {
    render(<CopyButton text="curl -sL solvr.dev/install.sh | bash" report={{ surface: 'skill', item: 'install_command' }} />);
    fireEvent.click(screen.getByRole('button'));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('code_copy', { surface: 'skill', item: 'install_command' });
    expectNoCopiedText();
  });

  it('waits for the clipboard: nothing is reported while the browser is still deciding', async () => {
    let allow: () => void = () => {};
    writeText.mockReturnValue(new Promise<void>((resolve) => (allow = resolve)));
    render(<CopyButton text="x" report={{ surface: 'mcp', item: 'mcp_config' }} />);

    fireEvent.click(screen.getByRole('button'));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();

    await act(async () => allow());
    expect(track).toHaveBeenCalledTimes(1);
  });

  it('reports nothing when the browser refuses the clipboard, or has none', async () => {
    writeText.mockRejectedValue(new Error('denied'));
    const { unmount } = render(<CopyButton text="x" report={{ surface: 'mcp', item: 'mcp_config' }} />);
    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});
    unmount();

    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true });
    render(<CopyButton text="x" report={{ surface: 'mcp', item: 'mcp_config' }} />);
    fireEvent.click(screen.getByRole('button'));
    await act(async () => {});

    expect(track).not.toHaveBeenCalled();
  });

  it('reports each press that copied, once', async () => {
    render(<CopyButton text="x" report={{ surface: 'api_docs', item: 'base_url' }} />);
    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(2));
  });

  it('cannot be placed without saying what it copies', () => {
    // @ts-expect-error report is required: a Copy button nobody hears about is a mistake
    render(<CopyButton text="x" />);
  });
});

describe('/skill', () => {
  it('names the install command at the top of the page', async () => {
    render(<SkillHero />);
    expect(await pressEveryCopy()).toEqual([{ surface: 'skill', item: 'install_command' }]);
    expectNoCopiedText();
  });

  it('names the two commands of the install section', async () => {
    render(<SkillInstall />);
    expect(await pressEveryCopy()).toEqual([
      { surface: 'skill', item: 'install_command' },
      { surface: 'skill', item: 'git_clone' },
    ]);
    expectNoCopiedText();
  });

  it('names the SKILL.md preview', async () => {
    render(<SkillPreview />);
    expect(await pressEveryCopy()).toEqual([{ surface: 'skill', item: 'skill_md' }]);
    expectNoCopiedText();
  });
});

describe('/mcp', () => {
  it('names the hosted config and the Claude Code command', async () => {
    render(<McpHero />);
    expect(await pressEveryCopy()).toEqual([
      { surface: 'mcp', item: 'mcp_config' },
      { surface: 'mcp', item: 'claude_mcp_add' },
    ]);
    expectNoCopiedText();
  });

  it('names the command that carries the key header', async () => {
    render(<McpSetup />);
    expect(await pressEveryCopy()).toEqual([{ surface: 'mcp', item: 'claude_mcp_add_with_key' }]);
    expectNoCopiedText();
  });
});

describe('/api-docs', () => {
  it('names the base URL and the quick start', async () => {
    render(<ApiHero />);
    expect(await pressEveryCopy()).toEqual([
      { surface: 'api_docs', item: 'base_url' },
      { surface: 'api_docs', item: 'quick_start' },
    ]);
    expectNoCopiedText();
  });

  it('names each step of the quickstart', async () => {
    render(<ApiQuickstart />);
    expect(await pressEveryCopy()).toEqual([
      { surface: 'api_docs', item: 'quickstart_step_01' },
      { surface: 'api_docs', item: 'quickstart_step_02' },
      { surface: 'api_docs', item: 'quickstart_step_03' },
      { surface: 'api_docs', item: 'quickstart_step_04' },
    ]);
    expectNoCopiedText();
  });

  it('names the install line and the example of the SDK tab that is showing', async () => {
    render(<ApiSdks />);
    expect(await pressEveryCopy()).toEqual([
      { surface: 'api_docs', item: 'sdk_install_go' },
      { surface: 'api_docs', item: 'sdk_example_go' },
    ]);

    track.mockClear();
    writeText.mockClear();
    fireEvent.click(screen.getByRole('button', { name: 'CLI' }));
    expect(await pressEveryCopy()).toEqual([
      { surface: 'api_docs', item: 'sdk_install_cli' },
      { surface: 'api_docs', item: 'sdk_example_cli' },
    ]);
    expectNoCopiedText();
  });

  it('names the two MCP blocks', async () => {
    render(<ApiMcp />);
    expect(await pressEveryCopy()).toEqual([
      { surface: 'api_docs', item: 'mcp_config' },
      { surface: 'api_docs', item: 'claude_mcp_add_with_key' },
    ]);
    expectNoCopiedText();
  });

  it("names an endpoint's example response", async () => {
    render(<ApiEndpoints />);
    const firstEndpoint = screen.getAllByText('/search', { selector: 'code' })[0];
    fireEvent.click(firstEndpoint.closest('button')!);

    expect(await pressEveryCopy()).toEqual([{ surface: 'api_docs', item: 'endpoint_response' }]);
    expectNoCopiedText();
  });
});

describe('the API playground: what is copied there can hold the token that was typed', () => {
  const endpoint = { method: 'GET' as const, path: '/posts/{id}', description: 'Get a post', auth: 'both' as const, params: [{ name: 'id', type: 'string', required: true, description: 'Post id' }], response: '{}' };

  it('reports the curl copy with the event name and the surface only: no item, no text', async () => {
    render(<ApiPlayground endpoint={endpoint} isOpen onClose={() => {}} />);
    fireEvent.change(screen.getByPlaceholderText(/solvr_|token|key/i), { target: { value: 'solvr_sk_live_SECRETSECRET' } });

    fireEvent.click(copyButtons()[0]);
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(writeText.mock.calls[0][0]).toContain('solvr_sk_live_SECRETSECRET');
    expect(track.mock.calls[0]).toEqual(['code_copy', { surface: 'api_playground' }]);
    expect(JSON.stringify(track.mock.calls)).not.toMatch(/SECRET|solvr_sk|posts|curl/);
  });

  it('says Copied, and reports, only once the clipboard took the command', async () => {
    writeText.mockRejectedValue(new Error('denied'));
    render(<ApiPlayground endpoint={endpoint} isOpen onClose={() => {}} />);

    fireEvent.click(copyButtons()[0]);
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
    expect(copyButtons()[0]).toHaveTextContent('Copy');
  });
});

describe('/how-it-works, /amcp and /ipfs', () => {
  it('names the API example of /how-it-works', async () => {
    render(<HowSolvr />);
    expect(await pressEveryCopy()).toEqual([{ surface: 'how_it_works', item: 'api_example' }]);
    expectNoCopiedText();
  });

  it('names the quickstart of /amcp', async () => {
    render(<AmcpRecovery />);
    expect(await pressEveryCopy()).toEqual([{ surface: 'amcp', item: 'quickstart' }]);
    expectNoCopiedText();
  });

  it('names each of the four calls of /ipfs', async () => {
    render(<IpfsApi />);
    expect(await pressEveryCopy()).toEqual([
      { surface: 'ipfs', item: 'upload' },
      { surface: 'ipfs', item: 'pin' },
      { surface: 'ipfs', item: 'list' },
      { surface: 'ipfs', item: 'status' },
    ]);
    expectNoCopiedText();
  });
});

// Every block that renders a Copy button is given a report in the source: a new copy
// button fails the type check without one, and this keeps the items stable ids.
describe('every copy block in the source', () => {
  const ROOT = join(__dirname, '..');
  const sources: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        if (entry !== 'node_modules' && !entry.startsWith('.')) walk(full);
      } else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) {
        sources.push(full);
      }
    }
  };
  for (const dir of ['app', 'components']) walk(join(ROOT, dir));

  it('says what it copies', () => {
    const missing: string[] = [];
    let blocks = 0;
    for (const file of sources) {
      const src = readFileSync(file, 'utf8');
      for (const match of src.matchAll(/<(CopyButton|HeroCode|CodeTile|CommandTile)\b[^>]*>/g)) {
        blocks++;
        if (!/\breport=/.test(match[0])) missing.push(`${relative(ROOT, file).split(sep).join('/')}: <${match[1]}>`);
      }
    }
    expect(blocks).toBeGreaterThan(15);
    expect(missing).toEqual([]);
  });

  it('uses lowercase ids for surface and item, never a sentence', () => {
    for (const file of sources) {
      const src = readFileSync(file, 'utf8');
      for (const match of src.matchAll(/report=\{\{\s*surface:\s*["']([^"']+)["'],\s*item:\s*["']([^"']+)["']\s*\}\}/g)) {
        expect(match[1], file).toMatch(/^[a-z][a-z0-9_]*$/);
        expect(match[2], file).toMatch(/^[a-z][a-z0-9_]*$/);
      }
    }
  });
});
