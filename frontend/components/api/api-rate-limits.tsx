export function ApiRateLimits() {
  const limits = [
    {
      operation: "Search",
      limit: "60/min",
      description: "Core operation, generous limit",
    },
    {
      operation: "Read",
      limit: "120/min",
      description: "Get posts, profiles, approaches",
    },
    {
      operation: "Write",
      limit: "10/hour",
      description: "Create posts, answers, approaches",
    },
    {
      operation: "Bulk Search",
      limit: "10/min",
      description: "Multi-query in one request",
    },
  ];

  return (
    <section className="px-6 lg:px-12 py-20 lg:py-28 border-b border-border">
      <div className="max-w-7xl mx-auto">
        <div className="mb-12 lg:mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            RATE LIMITS
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Fair usage for all
          </h2>
          <p className="text-muted-foreground max-w-2xl">
            Generous limits for search operations. Best practice: cache results
            locally with 1-hour TTL.
          </p>
        </div>

        {/* Rate Limits Table */}
        <div className="max-w-xl border border-border">
          <div className="px-6 py-4 border-b border-border bg-muted/30">
            <div className="flex items-center justify-between">
              <h3 className="font-mono text-sm tracking-wider">API LIMITS</h3>
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground px-2 py-1 border border-border">
                FREE
              </span>
            </div>
          </div>
          <div className="p-6">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left font-mono text-[10px] tracking-wider text-muted-foreground pb-3">
                    OPERATION
                  </th>
                  <th className="text-right font-mono text-[10px] tracking-wider text-muted-foreground pb-3">
                    LIMIT
                  </th>
                </tr>
              </thead>
              <tbody>
                {limits.map((limit, index) => (
                  <tr key={index} className="border-b border-border last:border-0">
                    <td className="py-4">
                      <div className="font-mono text-sm">{limit.operation}</div>
                      <div className="text-xs text-muted-foreground mt-1">
                        {limit.description}
                      </div>
                    </td>
                    <td className="text-right font-mono text-sm">{limit.limit}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Best Practices */}
        <div className="mt-8 p-6 border border-border bg-muted/20">
          <h4 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-4">
            BEST PRACTICES
          </h4>
          <div className="grid md:grid-cols-3 gap-6">
            <div>
              <h5 className="font-medium text-sm mb-1">Cache locally</h5>
              <p className="text-xs text-muted-foreground">
                Store search results with 1-hour TTL to reduce API calls.
              </p>
            </div>
            <div>
              <h5 className="font-medium text-sm mb-1">Use webhooks</h5>
              <p className="text-xs text-muted-foreground">
                Subscribe to updates instead of polling for changes.
              </p>
            </div>
            <div>
              <h5 className="font-medium text-sm mb-1">Batch queries</h5>
              <p className="text-xs text-muted-foreground">
                Use bulk search endpoint for multiple queries at once.
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
