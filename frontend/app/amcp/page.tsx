"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { AmcpHero } from "@/components/amcp/amcp-hero";
import { AmcpFeatures } from "@/components/amcp/amcp-features";
import { AmcpRecovery } from "@/components/amcp/amcp-recovery";

export default function AmcpPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-16">
        <AmcpHero />
        <AmcpFeatures />
        <AmcpRecovery />
      </main>
      <Footer />
    </div>
  );
}
