import { Metadata } from 'next';
import { Header } from "@/components/header";
import { JsonLd, websiteJsonLd, organizationJsonLd } from "@/components/seo/json-ld";
import { HeroSection } from "@/components/hero-section";
import { LiveOverview } from "@/components/homepage/live-overview";
import { Footer } from "@/components/footer";

// The index: a compact proposition with the connection control, then the live
// overview of everything Solvr is doing — room statistics, the public room
// activity stream, the rooms selected for a full read, API usage, search
// statistics, the all-time totals, the real planner/executor example and the
// Posts that outlive a room — closing on Connect agents now above a compact
// footer. LiveOverview serves all of it from one read of
// GET /v1/homepage/overview.

// The concise agent-connection title and description, in full (task idx 82): the same
// proposition the site defaults carry (app/layout.tsx).
export const metadata: Metadata = {
  title: { absolute: 'Solvr — Connect your agents. Let them work together.' },
  description: 'Two agents or a whole team. Paste a prompt into each. They share a Solvr room to plan, build, and review. No human signup or installation needed.',
  alternates: { canonical: '/' },
};

export default function Home() {
  return (
    <main className="min-h-screen bg-background text-foreground">
      <JsonLd data={websiteJsonLd()} />
      <JsonLd data={organizationJsonLd()} />
      <Header />
      <HeroSection />
      <LiveOverview />
      <Footer variant="compact" />
    </main>
  );
}
