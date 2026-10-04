"use client";

import { Check, X, Minus } from "lucide-react";
import { MarketingSection } from "@/components/page/marketing";

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

const TH = "pb-3 font-mono text-[11px] font-normal uppercase tracking-[0.18em] text-muted-foreground";

export function HowHonesty() {
  return (
    <MarketingSection
      heading={<>What we don&apos;t do (yet)</>}
      intro={
        <>
          Solvr solves a piece of the problem, not the whole thing. Here&apos;s what the research
          proposes vs. what we actually have today.
        </>
      }
    >
      {/* Comparison Table */}
      <table className="w-full table-fixed">
        <colgroup>
          <col />
          <col className="w-20 sm:w-28" />
          <col className="w-20 sm:w-28" />
        </colgroup>
        <thead>
          <tr className="border-b border-foreground">
            <th scope="col" className={`${TH} text-left`}>CAPABILITY</th>
            <th scope="col" className={`${TH} text-center`}>PAPER</th>
            <th scope="col" className={`${TH} text-center`}>SOLVR</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border border-b border-border">
          {comparisons.map((row) => (
            <tr key={row.feature}>
              <th scope="row" className="py-3.5 pr-4 text-left text-sm font-normal sm:text-base">{row.feature}</th>
              <td className="py-3.5">
                <Check size={16} className="mx-auto text-muted-foreground" />
              </td>
              <td className="py-3.5">
                <div className="flex items-center justify-center gap-2">
                  {row.solvr === true && <Check size={16} className="text-foreground" />}
                  {row.solvr === false && <X size={16} className="text-muted-foreground/50" />}
                  {row.solvr === "partial" && (
                    <>
                      <Minus size={16} className="text-muted-foreground" />
                      <span className="hidden font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground sm:inline">REP</span>
                    </>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <p className="mt-6 text-sm text-muted-foreground">
        We&apos;re building the foundation. The rest comes as the community grows.
      </p>
    </MarketingSection>
  );
}
