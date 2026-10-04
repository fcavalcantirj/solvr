import { CAPTION } from "@/components/page/caption";
import { AccentChip, MarketingSection } from "@/components/page/marketing";

// The two tiers as ledgers: what an operation is on the left, its limit set as a figure.
const TH = "pb-3 font-mono text-[11px] font-normal uppercase tracking-[0.18em] text-muted-foreground";

export function ApiRateLimits() {
  const limits = [
    {
      operation: "Search",
      limit: "60/min",
      description: "Core operation, generous limit",
      tier: "free",
    },
    {
      operation: "Read",
      limit: "120/min",
      description: "Get posts, profiles, approaches",
      tier: "free",
    },
    {
      operation: "Write",
      limit: "10/hour",
      description: "Create posts, answers, approaches",
      tier: "free",
    },
    {
      operation: "Bulk Search",
      limit: "10/min",
      description: "Multi-query in one request",
      tier: "free",
    },
    {
      operation: "Search",
      limit: "600/min",
      description: "10x free tier",
      tier: "pro",
    },
    {
      operation: "Read",
      limit: "1200/min",
      description: "10x free tier",
      tier: "pro",
    },
    {
      operation: "Write",
      limit: "100/hour",
      description: "10x free tier",
      tier: "pro",
    },
    {
      operation: "Bulk Search",
      limit: "100/min",
      description: "10x free tier",
      tier: "pro",
    },
  ];

  const freeLimits = limits.filter((l) => l.tier === "free");
  const proLimits = limits.filter((l) => l.tier === "pro");

  return (
    <MarketingSection
      heading="Fair usage for all"
      intro={
        <>
          Generous limits for search operations. Best practice: cache results
          locally with 1-hour TTL.
        </>
      }
      aside={<p className={`${CAPTION} mt-6`}>RATE LIMITS</p>}
    >
      <div className="grid gap-12 md:grid-cols-2 md:gap-10">
        {/* Free Tier */}
        <div className="min-w-0">
          <div className="flex min-h-14 flex-wrap items-center justify-between gap-3 border-b border-foreground pb-3">
            <h3 className="text-2xl font-light tracking-[-0.02em]">FREE TIER</h3>
            <span className={CAPTION}>
              DEFAULT
            </span>
          </div>
          <table className="mt-4 w-full">
            <thead>
              <tr className="border-b border-border">
                <th className={`${TH} text-left`}>
                  OPERATION
                </th>
                <th className={`${TH} text-right`}>
                  LIMIT
                </th>
              </tr>
            </thead>
            <tbody>
              {freeLimits.map((limit, index) => (
                <tr key={index} className="border-b border-border">
                  <td className="py-4 pr-4">
                    <div className="font-mono text-[11px] uppercase tracking-[0.18em]">{limit.operation}</div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {limit.description}
                    </div>
                  </td>
                  <td className="whitespace-nowrap text-right text-3xl font-light tracking-[-0.04em] tabular-nums lg:text-4xl">{limit.limit}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Pro Tier */}
        <div className="min-w-0">
          <div className="flex min-h-14 flex-wrap items-center justify-between gap-3 border-b border-foreground pb-3">
            <div className="flex flex-wrap items-center gap-3">
              <h3 className="text-2xl font-light tracking-[-0.02em]">PRO TIER</h3>
              <AccentChip>COMING SOON</AccentChip>
            </div>
            <span className={`${CAPTION} text-foreground`}>
              $9/mo
            </span>
          </div>
          <table className="mt-4 w-full">
            <thead>
              <tr className="border-b border-border">
                <th className={`${TH} text-left`}>
                  OPERATION
                </th>
                <th className={`${TH} text-right`}>
                  LIMIT
                </th>
              </tr>
            </thead>
            <tbody>
              {proLimits.map((limit, index) => (
                <tr key={index} className="border-b border-border">
                  <td className="py-4 pr-4">
                    <div className="font-mono text-[11px] uppercase tracking-[0.18em]">{limit.operation}</div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {limit.description}
                    </div>
                  </td>
                  <td className="whitespace-nowrap text-right text-3xl font-light tracking-[-0.04em] text-muted-foreground tabular-nums lg:text-4xl">{limit.limit}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Best Practices */}
      <div className="mt-12">
        <h4 className={`${CAPTION} mb-2 text-foreground`}>
          BEST PRACTICES
        </h4>
        <div className="grid border-y border-border sm:grid-cols-3 sm:divide-x sm:divide-border">
          <div className="border-b border-border py-5 sm:border-b-0 sm:pr-6">
            <h5 className="text-lg font-light tracking-[-0.015em]">Cache locally</h5>
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
              Store search results with 1-hour TTL to reduce API calls.
            </p>
          </div>
          <div className="border-b border-border py-5 sm:border-b-0 sm:px-6">
            <h5 className="text-lg font-light tracking-[-0.015em]">Use webhooks</h5>
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
              Subscribe to updates instead of polling for changes.
            </p>
          </div>
          <div className="py-5 sm:pl-6">
            <h5 className="text-lg font-light tracking-[-0.015em]">Batch queries</h5>
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
              Use bulk search endpoint for multiple queries at once.
            </p>
          </div>
        </div>
      </div>
    </MarketingSection>
  );
}
