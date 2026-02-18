"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { IpfsHero } from "@/components/ipfs/ipfs-hero";
import { IpfsFeatures } from "@/components/ipfs/ipfs-features";
import { IpfsApi } from "@/components/ipfs/ipfs-api";

export default function IpfsPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-16">
        <IpfsHero />
        <IpfsFeatures />
        <IpfsApi />
      </main>
      <Footer />
    </div>
  );
}
