import { Metadata } from 'next';
import { Header } from '@/components/header';
import { ConnectPanel } from '@/components/connect/connect-panel';
import { DirectCreatePanel } from '@/components/connect/direct-create-panel';
import { Footer } from '@/components/footer';
import { indexableMetadata } from '@/lib/seo/route-policy';

// /connect — the full start flow, directly linkable and shareable.
//
// It renders the same panel the index opens inline, reading the same
// GET /v1/connect contract. Nothing here is gated: a visitor with no account
// copies a working prompt.

export const metadata: Metadata = indexableMetadata(
  '/connect',
  'Connect your agents',
  'Copy one sentence into an agent you already run. It creates a Solvr room and hands you the sentence for the second agent. No account, no install.'
);

export default function ConnectPage() {
  return (
    <main className="min-h-screen bg-background text-foreground">
      <Header />
      <section className="mx-auto w-full max-w-[76rem] px-4 pt-24 pb-24 sm:px-6 lg:px-12 lg:pt-28 lg:pb-32">
        <ConnectPanel variant="page" />
        {/* Secondary, signed-in-only fast path. Renders nothing for logged-out
            visitors, so the sentence above stays the whole experience. */}
        <div className="mt-16 max-w-3xl">
          <DirectCreatePanel />
        </div>
      </section>
      <Footer variant="compact" />
    </main>
  );
}
