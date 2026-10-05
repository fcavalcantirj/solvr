import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { JsonLd, breadcrumbJsonLd } from "@/components/seo/json-ld";
import { GuideArticle } from "@/components/docs/guide-article";
import { guidePath, workflowGuide } from "@/lib/docs/workflow-guides";
import { linkPreview } from "@/lib/seo/link-preview";
import { getConnectExamples } from "@/lib/connect-examples-server";
import type { APIConnectPreset, APIConnectStart } from "@/lib/api-types";

// A workflow guide (task idx 84, SPEC.md 27.5). One component renders all nine from their
// data (lib/docs/workflow-guides.ts), on the server: what the guide is for, the API's
// sentence for its use case, the steps, the room of a real run, what to do when something
// goes wrong, and the run record. The sentence is never typed here: it is read from
// GET /v1/connect/examples, and the backend guide tests run exactly that sentence
// (internal/api/router_guides_http_test.go).

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

export const dynamic = "force-dynamic";

type Params = Promise<{ slug: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const guide = workflowGuide((await params).slug);
  if (!guide) return {};
  const path = guidePath(guide.slug);
  return {
    title: guide.title,
    description: guide.description,
    alternates: { canonical: path },
    // Its own link preview: without one the guide would show the guides index's, address included.
    ...linkPreview({ title: guide.title, description: guide.description, path }),
  };
}

export default async function WorkflowGuidePage({ params }: { params: Params }) {
  const guide = workflowGuide((await params).slug);
  if (!guide) notFound();
  const sentence =
    guide.kind === "resume"
      ? await liveSentence(guide.preset)
      : ((await getConnectExamples())?.find((p) => p.value === guide.preset) ?? null);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "Docs", path: "/docs" },
          { name: "Guides", path: "/docs/guides" },
          { name: guide.title, path: guidePath(guide.slug) },
        ])}
      />
      <Header />
      <main className="pt-20">
        <GuideArticle guide={guide} sentence={sentence} />
      </main>
      <Footer />
    </div>
  );
}

// liveSentence reads the first sentence the resume guide shows, as the API serves it now. A
// guide stays useful without it, so a failure leaves the sentence out and the page points
// at Connect. This read happens on the server, so it asks for no flow (flow=none): the
// page's HTML can be cached and shared, and no browser step stands behind a code minted
// for it. The sentence then carries the plain skill link, like the examples.
async function liveSentence(preset: string): Promise<APIConnectPreset | null> {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/connect?preset=${preset}&visibility=public&flow=none`, { cache: "no-store" });
    if (!res.ok) return null;
    const json = (await res.json()) as { data?: Partial<APIConnectStart> } | null;
    return json?.data?.presets?.find((p) => p.selected) ?? null;
  } catch {
    return null;
  }
}
