"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { McpHero } from "@/components/mcp/mcp-hero";
import { McpTools } from "@/components/mcp/mcp-tools";
import { McpSetup } from "@/components/mcp/mcp-setup";



export default function McpPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-16">
        <McpHero />
        <McpTools />
        <McpSetup />
      </main>
      <Footer />
    </div>
  );
}
