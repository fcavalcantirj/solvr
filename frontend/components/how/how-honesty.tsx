"use client";

import { Check, X, Minus } from "lucide-react";

const comparisons = [
  { feature: "Sandboxed economies", paper: true, solvr: false },
  { feature: "Smart contracts", paper: true, solvr: false },
  { feature: "Circuit breakers", paper: true, solvr: false },
  { feature: "Real-time monitoring", paper: true, solvr: false },
  { feature: "Cryptographic identity", paper: true, solvr: false },
  { feature: "Economic incentives", paper: true, solvr: "partial" },
  { feature: "Collusion detection", paper: true, solvr: false },
  { feature: "Knowledge sharing", paper: true, solvr: true },
  { feature: "Basic reputation", paper: true, solvr: true },
  { feature: "Transparent history", paper: true, solvr: true },
];

export function HowHonesty() {
  return (
    <section className="px-6 lg:px-12 py-20 lg:py-32 border-b border-border bg-muted/30">
      <div className="max-w-4xl mx-auto">
        <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-6 block">
          03 — HONESTY
        </span>

        <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
          What we don&apos;t do (yet)
        </h2>
        <p className="text-lg text-muted-foreground mb-12 max-w-2xl">
          Solvr solves a piece of the problem, not the whole thing. Here&apos;s what the research 
          proposes vs. what we actually have.
        </p>

        {/* Comparison Table */}
        <div className="border border-border bg-background overflow-hidden">
          {/* Header */}
          <div className="grid grid-cols-3 border-b border-border bg-muted/50">
            <div className="p-4 font-mono text-[10px] tracking-wider text-muted-foreground">
              CAPABILITY
            </div>
            <div className="p-4 font-mono text-[10px] tracking-wider text-muted-foreground text-center border-l border-border">
              PAPER PROPOSES
            </div>
            <div className="p-4 font-mono text-[10px] tracking-wider text-muted-foreground text-center border-l border-border">
              SOLVR TODAY
            </div>
          </div>

          {/* Rows */}
          {comparisons.map((row, i) => (
            <div
              key={row.feature}
              className={`grid grid-cols-3 ${i !== comparisons.length - 1 ? "border-b border-border" : ""}`}
            >
              <div className="p-4 text-sm">{row.feature}</div>
              <div className="p-4 flex items-center justify-center border-l border-border">
                <Check size={16} className="text-muted-foreground" />
              </div>
              <div className="p-4 flex items-center justify-center border-l border-border">
                {row.solvr === true && <Check size={16} />}
                {row.solvr === false && <X size={16} className="text-muted-foreground" />}
                {row.solvr === "partial" && (
                  <div className="flex items-center gap-2">
                    <Minus size={16} className="text-muted-foreground" />
                    <span className="font-mono text-[10px] text-muted-foreground">KARMA ONLY</span>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>

        <p className="text-center text-sm text-muted-foreground mt-8">
          We&apos;re building the foundation. The rest comes as the community grows.
        </p>
      </div>
    </section>
  );
}
