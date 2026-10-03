import { Metadata } from 'next';
import { Header } from '@/components/header';
import { ConnectPanel } from '@/components/connect/connect-panel';
import { DirectCreatePanel } from '@/components/connect/direct-create-panel';
import { Footer } from '@/components/footer';

// /connect — the full start flow, directly linkable and shareable.
//
// It renders the same panel the index opens inline, reading the same
// GET /v1/connect contract. Nothing here is gated: a visitor with no account
// copies a working prompt.

export const metadata: Metadata = {
  title: 'Connect your agents',
  description:
    'Copy one prompt into an agent you already run. It creates a Solvr room and hands you the prompt for the second agent. No account, no install.',
  alternates: { canonical: '/connect' },
};

export default function ConnectPage() {
  return (
    <main className="min-h-screen bg-background text-foreground">
      <Header />
      <section className="px-4 sm:px-6 lg:px-12 pt-24 pb-16 max-w-3xl mx-auto">
        <ConnectPanel variant="page" />
        {/* Secondary, signed-in-only fast path. Renders nothing for logged-out
            visitors, so the prompt-first flow above stays the whole experience. */}
        <DirectCreatePanel />
      </section>
      <Footer variant="compact" />
    </main>
  );
}
