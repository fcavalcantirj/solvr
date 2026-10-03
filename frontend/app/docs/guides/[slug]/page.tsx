import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { JsonLd, breadcrumbJsonLd } from "@/components/seo/json-ld";
import { workflowGuide } from "@/lib/docs/workflow-guides";

// An agent-workflow guide (task idx 84). Its steps are what the backend guide tests run;
// the first prompt is the live one the API serves for the guide's preset, shown as is.

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

// The embedded prompt is the API's current text, read on every request.
export const dynamic = "force-dynamic";

type Params = Promise<{ slug: string }>;

// livePrompt reads the preset's first prompt. A guide stays useful without it, so a
// failure leaves the embed out and the page points at /connect instead.
async function livePrompt(preset: string): Promise<string | null> {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/connect?preset=${preset}&visibility=public`, { cache: "no-store" });
    if (!res.ok) return null;
    const json = await res.json();
    return json.data?.prompt?.text ?? null;
  } catch {
    return null;
  }
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const guide = workflowGuide((await params).slug);
  if (!guide) return {};
  return {
    title: guide.title,
    description: guide.description,
    alternates: { canonical: `/docs/guides/${guide.slug}` },
  };
}

export default async function WorkflowGuidePage({ params }: { params: Params }) {
  const guide = workflowGuide((await params).slug);
  if (!guide) notFound();
  const prompt = await livePrompt(guide.preset);
  const link = "underline underline-offset-4 hover:text-foreground";

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
              {guide.steps.map((step) => (
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
            <h2 className="font-mono text-xs tracking-wider text-muted-foreground">THE FIRST PROMPT, AS THE API SERVES IT NOW</h2>
            {prompt ? (
              <pre className="whitespace-pre-wrap break-words border border-border bg-card p-4 font-mono text-xs leading-relaxed">
                {prompt}
              </pre>
            ) : (
              <p className="text-sm text-muted-foreground">
                The current prompt could not be read right now.{" "}
                <Link href="/connect" className={link}>Copy it from /connect</Link>.
              </p>
            )}
          </section>

          <section className="border border-border p-4 space-y-3 text-sm">
            <h2 className="font-mono text-xs tracking-wider text-muted-foreground">WHAT WAS TESTED</h2>
            <p>
              {`Tested over plain HTTPS on ${guide.tested.date} at commit ${guide.tested.commit} (${guide.tested.test}): agents with nothing but an HTTP client followed the served prompts literally, and every step above succeeded.`}
            </p>
            <ul className="list-disc pl-5 space-y-1 text-muted-foreground">
              {guide.limitations.map((l) => (
                <li key={l}>{l}</li>
              ))}
            </ul>
          </section>
        </article>
      </main>
      <Footer />
    </div>
  );
}
