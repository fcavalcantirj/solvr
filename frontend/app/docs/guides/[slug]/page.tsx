import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { JsonLd, breadcrumbJsonLd } from "@/components/seo/json-ld";
import { GuidePrompt } from "@/components/prompt/guide-prompt";
import { workflowGuide, type WorkflowGuide } from "@/lib/docs/workflow-guides";
import { linkPreview } from "@/lib/seo/link-preview";
import { getConnectExamples } from "@/lib/connect-examples-server";
import type { APIConnectPreset } from "@/lib/api-types";

// An agent-workflow guide (task idx 84, v1.3.5): a title, one line, and the API's example
// sentence for the guide's use case, big and read-only. Nothing else: the sentence is the
// whole instruction, and the skill it points at teaches the agent the rest. The backend
// guide tests run exactly this sentence (internal/api/router_guides_http_test.go).
//
// The unlisted resume guide keeps its long-form content (ResumeGuide below).

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

export const dynamic = "force-dynamic";

type Params = Promise<{ slug: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const guide = workflowGuide((await params).slug);
  if (!guide) return {};
  const path = `/docs/guides/${guide.slug}`;
  return {
    title: guide.title,
    description: guide.description,
    alternates: { canonical: path },
    // Its own link preview: without one the guide would show the guides index's, address included.
    ...linkPreview({ title: guide.title, description: guide.description, path }),
  };
}

const crumb = "hover:text-foreground transition-colors";

export default async function WorkflowGuidePage({ params }: { params: Params }) {
  const guide = workflowGuide((await params).slug);
  if (!guide) notFound();
  const example = guide.listed ? (await getConnectExamples())?.find((p) => p.value === guide.preset) : undefined;
  const resumePrompt = guide.listed ? null : await livePrompt(guide.preset);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "Docs", path: "/docs" },
          { name: "Guides", path: "/docs/guides" },
          { name: guide.title, path: `/docs/guides/${guide.slug}` },
        ])}
      />
      <Header />
      <main className="pt-20">
        {guide.listed ? <UseCaseGuide guide={guide} example={example} /> : <ResumeGuide guide={guide} prompt={resumePrompt} />}
      </main>
      <Footer />
    </div>
  );
}

function UseCaseGuide({ guide, example }: { guide: WorkflowGuide; example?: APIConnectPreset }) {
  return (
    <article className="mx-auto w-full max-w-[76rem] px-4 pt-12 pb-24 sm:px-6 lg:px-12 lg:pt-16 lg:pb-32">
      <nav aria-label="Breadcrumb" className="font-mono text-xs uppercase tracking-[0.2em] text-muted-foreground space-x-2">
        <Link href="/docs" className={crumb}>Docs</Link>
        <span aria-hidden="true">/</span>
        <Link href="/docs/guides" className={crumb}>Guides</Link>
      </nav>
      <header className="mt-6 max-w-[44rem]">
        <h1 className="text-[1.625rem] font-normal leading-[1.2] tracking-[-0.02em] sm:text-[1.875rem]">{guide.title}</h1>
        <p className="mt-4 text-base leading-relaxed text-muted-foreground sm:text-lg">{guide.description}</p>
      </header>
      <div className="mt-12 lg:mt-14" data-testid="guide-prompt">
        {example ? (
          <GuidePrompt example={example} />
        ) : (
          <p className="text-muted-foreground">
            The sentence could not be read right now.{" "}
            <Link href={`/connect?preset=${guide.preset}`} className="text-foreground underline underline-offset-4">
              Copy it from Connect
            </Link>
            .
          </p>
        )}
      </div>
    </article>
  );
}

// livePrompt reads the plan-and-build sentence the resume guide shows. A guide stays
// useful without it, so a failure leaves the embed out and the page points at /connect.
// This read happens on the server, so it asks for no flow (flow=none): the page's HTML
// can be cached and shared, and no browser step stands behind a code minted for it. The
// sentence then carries the plain skill link, like the examples.
async function livePrompt(preset: string): Promise<string | null> {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/connect?preset=${preset}&visibility=public&flow=none`, { cache: "no-store" });
    if (!res.ok) return null;
    const json = await res.json();
    return json.data?.prompt?.text ?? null;
  } catch {
    return null;
  }
}

function ResumeGuide({ guide, prompt }: { guide: WorkflowGuide; prompt: string | null }) {
  const link = "underline underline-offset-4 hover:text-foreground";

  return (
    <article className="max-w-3xl mx-auto px-6 py-12 space-y-10">
      <nav aria-label="Breadcrumb" className="font-mono text-xs text-muted-foreground space-x-2">
        <Link href="/docs" className={link}>Docs</Link>
        <span aria-hidden="true">/</span>
        <Link href="/docs/guides" className={link}>Guides</Link>
      </nav>

      <header className="space-y-4">
        <h1 className="text-3xl sm:text-4xl font-light tracking-tight">{guide.title}</h1>
        <p className="text-muted-foreground leading-relaxed">{guide.description}</p>
      </header>

      <section className="space-y-4">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground">STEPS</h2>
        <ol className="list-decimal pl-6 space-y-3 leading-relaxed">
          {(guide.steps ?? []).map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
        <p>
          <Link href="/connect" className={`font-mono text-sm ${link}`}>
            Start at /connect
          </Link>
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground">THE FIRST SENTENCE, AS THE API SERVES IT NOW</h2>
        {prompt ? (
          <pre className="whitespace-pre-wrap break-words border border-border bg-card p-4 font-mono text-xs leading-relaxed">
            {prompt}
          </pre>
        ) : (
          <p className="text-sm text-muted-foreground">
            The current sentence could not be read right now.{" "}
            <Link href="/connect" className={link}>Copy it from /connect</Link>.
          </p>
        )}
      </section>

      {guide.tested ? (
        <section className="border border-border p-4 space-y-3 text-sm">
          <h2 className="font-mono text-xs tracking-wider text-muted-foreground">WHAT WAS TESTED</h2>
          <p>
            {`Tested over plain HTTPS on ${guide.tested.date} at commit ${guide.tested.commit} (${guide.tested.test}): agents with nothing but an HTTP client followed the served sentences and the skill literally, and every step above succeeded.`}
          </p>
          <ul className="list-disc pl-5 space-y-1 text-muted-foreground">
            {(guide.limitations ?? []).map((l) => (
              <li key={l}>{l}</li>
            ))}
          </ul>
        </section>
      ) : null}
    </article>
  );
}
