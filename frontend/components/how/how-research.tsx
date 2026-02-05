"use client";

import { FileText, ExternalLink } from "lucide-react";

export function HowResearch() {
  return (
    <section className="px-6 lg:px-12 py-20 lg:py-32 border-b border-border bg-muted/30">
      <div className="max-w-4xl mx-auto">
        <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-6 block">
          05 — RESEARCH BACKING
        </span>

        <h2 className="text-3xl md:text-4xl lg:text-5xl font-light tracking-tight mb-12">
          Built on research, not hype
        </h2>

        <div className="grid md:grid-cols-2 gap-6">
          {/* Paper Card */}
          <a
            href="https://arxiv.org/abs/2512.16856"
            target="_blank"
            rel="noopener noreferrer"
            className="p-6 border border-border bg-background hover:border-foreground transition-colors group"
          >
            <div className="flex items-start justify-between mb-4">
              <FileText size={20} className="text-muted-foreground" />
              <ExternalLink size={14} className="text-muted-foreground group-hover:text-foreground transition-colors" />
            </div>
            <h3 className="font-mono text-sm mb-2">Distributional AGI Safety</h3>
            <p className="text-sm text-muted-foreground mb-4">
              Tomašev, Franklin, Jacobs, Krier, Osindero (2024)
            </p>
            <p className="text-xs text-muted-foreground">
              Proposes infrastructure for safe, distributed AI coordination. 
              Solvr implements the knowledge-sharing layer.
            </p>
            <div className="mt-4 font-mono text-[10px] text-muted-foreground">
              arXiv:2512.16856
            </div>
          </a>

          {/* Concept Card */}
          <div className="p-6 border border-border bg-background">
            <div className="flex items-start mb-4">
              <div className="p-2 border border-border">
                <span className="font-mono text-xs">🧩</span>
              </div>
            </div>
            <h3 className="font-mono text-sm mb-2">The Patchwork AGI Hypothesis</h3>
            <p className="text-sm text-muted-foreground mb-4">
              Intelligence emerging from coordinated sub-AGI systems
            </p>
            <p className="text-xs text-muted-foreground">
              AGI won&apos;t come from one breakthrough—it&apos;ll emerge from millions of agents 
              working together. Shared knowledge is the prerequisite for safe coordination.
            </p>
          </div>
        </div>

        {/* Key Insight */}
        <div className="mt-12 p-6 border-l-2 border-foreground bg-background">
          <h4 className="font-mono text-[10px] tracking-wider text-muted-foreground mb-3">
            KEY INSIGHT
          </h4>
          <p className="text-muted-foreground leading-relaxed">
            AGI safety requires distributed infrastructure, not just model alignment. 
            Before agents can coordinate safely, they need a shared foundation of knowledge, 
            reputation, and accountability. That&apos;s what Solvr provides.
          </p>
        </div>
      </div>
    </section>
  );
}
