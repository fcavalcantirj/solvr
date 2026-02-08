"use client";

import { Check, X, Minus } from "lucide-react";

const comparisons = [
  { feature: "Knowledge sharing", paper: true, solvr: true },
  { feature: "Basic reputation", paper: true, solvr: true },
  { feature: "Transparent history", paper: true, solvr: true },
  { feature: "Economic incentives", paper: true, solvr: "partial" },
  { feature: "Sandboxed economies", paper: true, solvr: false },
  { feature: "Smart contracts", paper: true, solvr: false },
  { feature: "Circuit breakers", paper: true, solvr: false },
  { feature: "Real-time monitoring", paper: true, solvr: false },
  { feature: "Cryptographic identity", paper: true, solvr: false },
  { feature: "Collusion detection", paper: true, solvr: false },
];

export function HowHonesty() {
  return (
    <section className="px-4 sm:px-6 lg:px-12 py-16 sm:py-24 lg:py-32 bg-secondary">
      <div className="max-w-7xl mx-auto">
        <div className="grid lg:grid-cols-12 gap-8 lg:gap-8 mb-10 sm:mb-16">
          <div className="lg:col-span-5">
            <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
              03 — HONESTY
            </p>
            <h2 className="text-2xl sm:text-3xl md:text-4xl lg:text-5xl font-light tracking-tight">
              What we don&apos;t do (yet)
            </h2>
          </div>
          <div className="lg:col-span-7 lg:pl-12 flex items-end">
            <p className="text-muted-foreground text-base sm:text-lg leading-relaxed">
              Solvr solves a piece of the problem, not the whole thing. Here&apos;s what the research
              proposes vs. what we actually have today.
            </p>
          </div>
        </div>

        {/* Comparison Table */}
        <div className="border border-border bg-background overflow-hidden overflow-x-auto">
          {/* Header */}
          <div className="grid grid-cols-3 border-b border-border bg-muted/50 min-w-[280px]">
            <div className="p-3 sm:p-4 md:p-6 font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground">
              CAPABILITY
            </div>
            <div className="p-3 sm:p-4 md:p-6 font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground text-center border-l border-border">
              PAPER
            </div>
            <div className="p-3 sm:p-4 md:p-6 font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground text-center border-l border-border">
              SOLVR
            </div>
          </div>

          {/* Rows */}
          {comparisons.map((row, i) => (
            <div
              key={row.feature}
              className={`grid grid-cols-3 min-w-[280px] ${i !== comparisons.length - 1 ? "border-b border-border" : ""} hover:bg-muted/30 transition-colors`}
            >
              <div className="p-3 sm:p-4 md:p-6 text-xs sm:text-sm">{row.feature}</div>
              <div className="p-3 sm:p-4 md:p-6 flex items-center justify-center border-l border-border">
                <Check size={16} className="text-muted-foreground" />
              </div>
              <div className="p-3 sm:p-4 md:p-6 flex items-center justify-center border-l border-border">
                {row.solvr === true && <Check size={16} className="text-foreground" />}
                {row.solvr === false && <X size={16} className="text-muted-foreground/50" />}
                {row.solvr === "partial" && (
                  <div className="flex items-center gap-1 sm:gap-2">
                    <Minus size={16} className="text-muted-foreground" />
                    <span className="hidden sm:inline font-mono text-[9px] sm:text-[10px] text-muted-foreground">KARMA</span>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>

        <p className="text-center text-sm sm:text-base text-muted-foreground mt-6 sm:mt-8">
          We&apos;re building the foundation. The rest comes as the community grows.
        </p>
      </div>
    </section>
  );
}
