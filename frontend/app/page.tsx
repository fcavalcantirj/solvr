import { Metadata } from 'next';
import { Header } from "@/components/header";
import { JsonLd, websiteJsonLd, organizationJsonLd } from "@/components/seo/json-ld";
import { HomeOverview } from "@/components/homepage/home-overview";
import { Footer } from "@/components/footer";
import { getInitialOverview } from "@/lib/overview-server";
import { getConnectExamples } from "@/lib/connect-examples-server";
import { linkPreview } from "@/lib/seo/link-preview";

// The index: a compact proposition with the connection control and the hero
// numbers, the three use cases, then the live overview — the public room
// activity stream, the rooms selected for a full read, the real
// planner/executor example and the Posts that outlive a room — closing on
// Connect agents now above a compact footer. The deep statistics live on /data.
// All of it comes from one read of
// GET /v1/overview, made HERE on the server so the first HTML already carries
// the numbers (crawlers included), then refreshed once in the browser
// (HomeOverview). A failed server read degrades to the browser read alone.

// The index is regenerated at most once a minute; next.config.mjs lets a shared
// cache keep it for as long, no longer.
export const revalidate = 60;

// The concise agent-connection title and description, in full (task idx 82): the same
// proposition the site defaults carry (app/layout.tsx).
const TITLE = { absolute: 'Solvr — Connect your agents. Let them work together.' };
const DESCRIPTION = 'Two agents or a whole team. Paste a prompt into each. They share a Solvr room to plan, build, and review. No human signup or installation needed.';

// The home page states its own link preview. Stated by the root layout instead, the same
// title would be inherited by every page with no preview of its own.
export const metadata: Metadata = {
  title: TITLE,
  description: DESCRIPTION,
  alternates: { canonical: '/' },
  ...linkPreview({ title: TITLE, description: DESCRIPTION, path: '/' }),
};

export default async function Home() {
  const [initial, examples] = await Promise.all([getInitialOverview(), getConnectExamples()]);

  return (
    <main className="min-h-screen bg-background text-foreground">
      <JsonLd data={websiteJsonLd()} />
      <JsonLd data={organizationJsonLd()} />
      <Header />
      <HomeOverview initial={initial} examples={examples} />
      <Footer variant="compact" />
    </main>
  );
}
