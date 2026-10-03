import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The agent profile page renders the agent's name and bio from the API. An agent
// that deletes itself or is banned is refused by the API at once, so the page must
// ask the API on every request: a stored response would keep publishing the removed
// profile for an hour.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
const notFound = vi.fn(() => {
  throw new Error('NEXT_NOT_FOUND');
});
vi.mock('next/navigation', () => ({ notFound: () => notFound() }));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/agents/agent-profile-client', () => ({ AgentProfileClient: () => null }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, agentJsonLd: () => ({}) }));

import * as agentPage from './page';

const fetchMock = vi.fn();
const params = Promise.resolve({ id: 'agent_one' });

beforeEach(() => {
  fetchMock.mockReset();
  notFound.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('agent profile page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(agentPage.dynamic).toBe('force-dynamic');
    expect((agentPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('reads the agent from the API without the data cache', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: { agent: { id: 'agent_one', display_name: 'Listed Agent', bio: 'Bio' } } }),
    });

    const metadata = await agentPage.generateMetadata({ params });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/agents/agent_one');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
    expect(metadata.title).toBe('Listed Agent');
  });

  it('publishes nothing and answers 404 once the API refuses the agent', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 404, json: async () => ({}) });

    expect(await agentPage.generateMetadata({ params })).toEqual({});
    await expect(agentPage.default({ params })).rejects.toThrow('NEXT_NOT_FOUND');
    expect(notFound).toHaveBeenCalledTimes(1);
  });

  // Task idx 83: an API failure is not a missing agent. A real 404 tells crawlers to
  // drop the page; a failure must stay a retryable 5xx (an error the server renders).
  it('fails retryably, never as a 404, when the API fails', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    await expect(agentPage.default({ params })).rejects.toThrow(/503/);
    expect(notFound).not.toHaveBeenCalled();
  });

  it('fails retryably, never as a 404, when the API is unreachable', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    await expect(agentPage.default({ params })).rejects.toThrow(/unreachable/);
    expect(notFound).not.toHaveBeenCalled();
  });
});
