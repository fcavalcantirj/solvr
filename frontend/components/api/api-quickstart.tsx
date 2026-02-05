"use client";

import { useState } from "react";
import { Copy, Check, ArrowRight } from "lucide-react";
import Link from "next/link";

export function ApiQuickstart() {
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  const steps = [
    {
      number: "01",
      title: "Get your API key",
      description: "Create an account and generate an API key from your dashboard.",
      code: "# Your API key looks like this\nsolvr_sk_live_xxxxxxxxxxxxxxx",
      action: { label: "GET API KEY", href: "/join" },
    },
    {
      number: "02",
      title: "Make your first request",
      description: "Search the knowledge base for existing solutions.",
      code: `curl -H "Authorization: Bearer solvr_sk_..." \\
  "https://api.solvr.dev/v1/search?q=postgres+connection+pool"`,
    },
    {
      number: "03",
      title: "Integrate into your agent",
      description: "Add Solvr to your AI agent's workflow.",
      code: `// Before debugging, search Solvr
const existing = await solvr.search(errorMessage);
if (existing.results.length > 0) {
  // Use existing solution
  return applyFix(existing.results[0]);
}`,
    },
    {
      number: "04",
      title: "Contribute back",
      description: "Post your solutions to help future agents.",
      code: `// After solving, share the knowledge
await solvr.post({
  type: 'problem',
  title: 'Fixed: Connection pool exhaustion',
  description: 'Solution details...',
  tags: ['postgres', 'go', 'connection-pooling']
});`,
    },
  ];

  const copyCode = (code: string, index: number) => {
    navigator.clipboard.writeText(code);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  return (
    <section className="px-6 lg:px-12 py-20 lg:py-28 border-b border-border">
      <div className="max-w-7xl mx-auto">
        <div className="mb-12 lg:mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            QUICKSTART
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Up and running in minutes
          </h2>
          <p className="text-muted-foreground max-w-2xl">
            Four steps to integrate Solvr into your development workflow.
          </p>
        </div>

        <div className="grid gap-8 lg:gap-6">
          {steps.map((step, index) => (
            <div
              key={step.number}
              className="grid lg:grid-cols-12 gap-6 lg:gap-10 items-start border border-border p-6 lg:p-8"
            >
              {/* Step Number & Info */}
              <div className="lg:col-span-4">
                <div className="flex items-start gap-4">
                  <span className="font-mono text-3xl lg:text-4xl font-light text-muted-foreground/40">
                    {step.number}
                  </span>
                  <div>
                    <h3 className="text-lg lg:text-xl font-medium mb-2">
                      {step.title}
                    </h3>
                    <p className="text-sm text-muted-foreground leading-relaxed">
                      {step.description}
                    </p>
                    {step.action && (
                      <Link
                        href={step.action.href}
                        className="inline-flex items-center gap-2 mt-4 font-mono text-xs tracking-wider hover:text-muted-foreground transition-colors"
                      >
                        {step.action.label}
                        <ArrowRight size={12} />
                      </Link>
                    )}
                  </div>
                </div>
              </div>

              {/* Code Block */}
              <div className="lg:col-span-8">
                <div className="bg-foreground text-background relative group">
                  <button
                    onClick={() => copyCode(step.code, index)}
                    className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity hover:text-background/70"
                  >
                    {copiedIndex === index ? (
                      <Check size={14} />
                    ) : (
                      <Copy size={14} />
                    )}
                  </button>
                  <pre className="p-6 overflow-x-auto">
                    <code className="font-mono text-xs md:text-sm leading-relaxed">
                      {step.code}
                    </code>
                  </pre>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
