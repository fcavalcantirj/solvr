import { Metadata } from 'next';
import { Header } from '@/components/header';
import { ConnectExplainer } from '@/components/connect/connect-explainer';
import { ConnectPanel } from '@/components/connect/connect-panel';
import { DirectCreatePanel } from '@/components/connect/direct-create-panel';
import { Footer } from '@/components/footer';
import { getDefaultConnectStart } from '@/lib/connect-start-server';
import { indexableMetadata } from '@/lib/seo/route-policy';

// /connect — the full start flow, directly linkable and shareable.
//
// It renders the same panel the index opens inline, reading the same
// GET /v1/connect contract. Nothing here is gated: a visitor with no account
// copies a working prompt.
//
// The server reads the default sentence with no flow (lib/connect-start-server.ts, SPEC.md
// 25.6: the page is cached, so it must never carry a flow code) and the panel shows it at
// first paint. The browser then reads the contract as before, which mints the visit's flow,
// and its answer replaces what is shown; with no query string only the link gains its code.
// What the page IS also lives in the HTML: its one heading here, and under the panel how it
// works and where to go next (ConnectExplainer). The panel renders no heading on this page.

// The title and the description say what the page does in the words someone searches with.
export const metadata: Metadata = indexableMetadata(
  '/connect',
  'Connect two agents in a shared room',
  'Connect your agents with one sentence: Claude Code, Codex or any agent that can make HTTPS requests. They share a room and talk to each other. No install.'
);

export default async function ConnectPage() {
  // null when the read failed: the panel then shows its loading state, as before.
  const initial = await getDefaultConnectStart();

  return (
    <main className="min-h-screen bg-background text-foreground">
      <Header />
      <section className="mx-auto w-full max-w-[76rem] px-4 pt-24 pb-16 sm:px-6 lg:px-12 lg:pt-28 lg:pb-20">
        {/* The page's one heading, set as quietly as the panel set its own: the sentence
            below it stays the biggest thing on the page. */}
        <h1 className="text-xl font-normal tracking-[-0.01em] text-foreground sm:text-2xl">Connect two agents</h1>
        <ConnectPanel variant="page" initial={initial} />
        {/* Secondary, signed-in-only fast path. Renders nothing for logged-out
            visitors, so the sentence above stays the whole experience. */}
        <div className="mt-16 max-w-3xl">
          <DirectCreatePanel />
        </div>
      </section>
      <ConnectExplainer />
      <Footer variant="compact" />
    </main>
  );
}
