import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import type { APIOverviewAPIUsage } from '@/lib/api-types';
import { MetricGrid, SectionHeading } from './metric';

// API usage. The volumes are measured call counts served by the API itself,
// each one carrying the window it covers; the endpoint list is the API's own
// answer to "what does an agent actually call".

export function ApiUsageSection({ data }: { data: APIOverviewAPIUsage }) {
  return (
    <section
      data-testid="overview-section-api"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="API" heading={data.heading} intro={data.intro} />

        <div className="mt-12 grid lg:grid-cols-12 gap-8 lg:gap-12 items-start">
          <div className="lg:col-span-6">
            <MetricGrid metrics={data.metrics} />
          </div>

          <ul className="lg:col-span-6 border border-border divide-y divide-border">
            {data.endpoints.map((endpoint) => (
              <li
                key={`${endpoint.method} ${endpoint.path}`}
                data-testid="overview-endpoint"
                className="p-4 sm:p-5"
              >
                <p className="font-mono text-xs tracking-wider break-words">
                  <span className="text-muted-foreground">{endpoint.method}</span>{' '}
                  {endpoint.path}
                </p>
                <p className="text-sm text-muted-foreground leading-relaxed mt-1">
                  {endpoint.summary}
                </p>
              </li>
            ))}
          </ul>
        </div>

        <Link
          href={data.docs_url}
          className="group inline-flex items-center gap-3 mt-10 font-mono text-xs uppercase tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors"
        >
          {data.docs_label}
          <ArrowRight size={14} className="group-hover:translate-x-1 transition-transform" />
        </Link>
      </div>
    </section>
  );
}
