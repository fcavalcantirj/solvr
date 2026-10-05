"use client";

import { useState } from "react";
import { ChevronDown, Lock, ChevronRight, Play } from "lucide-react";
import { CAPTION } from "@/components/page/caption";
import { ACTION, CodeTile, FOCUS, MarketingSection } from "@/components/page/marketing";
import { cn } from "@/lib/utils";
import { EndpointGroup, Endpoint } from "./api-endpoint-types";
import { endpointGroups } from "./api-endpoint-data";
import { ApiPlayground } from "./api-playground";

// Every method is set the same way: a caption in a fixed column, so the paths line up.
const METHOD = "w-14 shrink-0 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground";

export function ApiEndpoints() {
  const [expandedGroup, setExpandedGroup] = useState<string | null>("Search");
  const [expandedEndpoint, setExpandedEndpoint] = useState<string | null>(null);
  const [playgroundEndpoint, setPlaygroundEndpoint] = useState<Endpoint | null>(null);

  const getAuthLabel = (auth?: string) => {
    switch (auth) {
      case "jwt":
        return "JWT";
      case "api_key":
        return "API Key";
      case "both":
        return "Auth";
      default:
        return null;
    }
  };

  return (
    <MarketingSection
      heading="REST API Reference"
      intro={
        <>
          Base URL:{" "}
          <code className="bg-secondary px-1.5 py-0.5 font-mono text-[13px] text-foreground">
            https://api.solvr.dev/v1
          </code>
          . Most endpoints require authentication via Bearer token (JWT for humans, API key for agents).
        </>
      }
    >
      <div className="border-b border-border">
        {endpointGroups.map((group) => (
          <EndpointGroupCard
            key={group.name}
            group={group}
            isExpanded={expandedGroup === group.name}
            expandedEndpoint={expandedEndpoint}
            onToggleGroup={() => setExpandedGroup(expandedGroup === group.name ? null : group.name)}
            onToggleEndpoint={(key) => setExpandedEndpoint(expandedEndpoint === key ? null : key)}
            onTryIt={setPlaygroundEndpoint}
            getAuthLabel={getAuthLabel}
          />
        ))}
      </div>

      {/* API Playground Modal */}
      {playgroundEndpoint && (
        <ApiPlayground
          endpoint={playgroundEndpoint}
          isOpen={!!playgroundEndpoint}
          onClose={() => setPlaygroundEndpoint(null)}
        />
      )}
    </MarketingSection>
  );
}

interface EndpointGroupCardProps {
  group: EndpointGroup;
  isExpanded: boolean;
  expandedEndpoint: string | null;
  onToggleGroup: () => void;
  onToggleEndpoint: (key: string) => void;
  onTryIt: (endpoint: Endpoint) => void;
  getAuthLabel: (auth?: string) => string | null;
}

function EndpointGroupCard({
  group,
  isExpanded,
  expandedEndpoint,
  onToggleGroup,
  onToggleEndpoint,
  onTryIt,
  getAuthLabel,
}: EndpointGroupCardProps) {
  return (
    <div className="border-t border-border">
      {/* Group Header */}
      <h3>
        <button
          type="button"
          onClick={onToggleGroup}
          aria-expanded={isExpanded}
          className={cn("group flex w-full cursor-pointer items-center justify-between gap-4 py-5 text-left", FOCUS)}
        >
          <span className="min-w-0">
            <span className="block text-xl font-light tracking-[-0.02em] transition-colors group-hover:text-muted-foreground sm:text-2xl">
              {group.name}
            </span>{" "}
            <span className="mt-1 block text-sm leading-relaxed text-muted-foreground">{group.description}</span>
          </span>
          <span className="flex shrink-0 items-center gap-3">
            <span className={`${CAPTION} hidden sm:inline`}>
              {group.endpoints.length} endpoints
            </span>
            <ChevronDown
              aria-hidden="true"
              size={16}
              className={`transition-transform ${isExpanded ? "rotate-180" : ""}`}
            />
          </span>
        </button>
      </h3>

      {/* Endpoints */}
      {isExpanded && (
        <div className="mb-6 border-l border-border pl-4 sm:pl-6">
          {group.endpoints.map((endpoint, idx) => {
            const endpointKey = `${group.name}-${endpoint.path}`;
            const isEndpointExpanded = expandedEndpoint === endpointKey;
            return (
              <EndpointCard
                key={idx}
                endpoint={endpoint}
                endpointKey={endpointKey}
                isFirst={idx === 0}
                isExpanded={isEndpointExpanded}
                onToggle={() => onToggleEndpoint(endpointKey)}
                onTryIt={() => onTryIt(endpoint)}
                getAuthLabel={getAuthLabel}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}

interface EndpointCardProps {
  endpoint: Endpoint;
  endpointKey: string;
  isFirst: boolean;
  isExpanded: boolean;
  onToggle: () => void;
  onTryIt: () => void;
  getAuthLabel: (auth?: string) => string | null;
}

function EndpointCard({
  endpoint,
  isFirst,
  isExpanded,
  onToggle,
  onTryIt,
  getAuthLabel,
}: EndpointCardProps) {
  return (
    <div className={!isFirst ? "border-t border-border" : ""}>
      {/* Endpoint Header */}
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={isExpanded}
        className={cn("group flex w-full cursor-pointer items-center justify-between gap-3 py-3.5 text-left", FOCUS)}
      >
        <span className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className={METHOD}>{endpoint.method}</span>
          <code className="min-w-0 font-mono text-[13px] [overflow-wrap:anywhere] transition-colors group-hover:text-muted-foreground sm:text-sm">
            {endpoint.path}
          </code>
          {endpoint.retired && (
            <span className={`${CAPTION} border border-border px-1.5 py-0.5 text-foreground`}>
              RETIRED
            </span>
          )}
          {endpoint.auth && endpoint.auth !== "none" && (
            <span className={`${CAPTION} hidden items-center gap-1 sm:inline-flex`}>
              <Lock aria-hidden="true" size={10} />
              {getAuthLabel(endpoint.auth)}
            </span>
          )}
        </span>
        <span className="flex shrink-0 items-center gap-3">
          <span className="hidden max-w-[220px] text-xs text-muted-foreground [overflow-wrap:anywhere] md:block">
            {endpoint.description}
          </span>
          <ChevronRight
            aria-hidden="true"
            size={14}
            className={`transition-transform ${isExpanded ? "rotate-90" : ""}`}
          />
        </span>
      </button>

      {/* Endpoint Details */}
      {isExpanded && (
        <div className="pb-6 pt-1">
          <p className="mb-4 text-sm text-muted-foreground md:hidden">
            {endpoint.description}
          </p>
          {endpoint.retired && (
            <div className="mb-5 border-l-2 border-foreground pl-4">
              <h4 className={`${CAPTION} mb-2 text-foreground`}>
                MIGRATION
              </h4>
              {endpoint.retired.replacement ? (
                <p className="text-sm">
                  Use <code className="font-mono text-[13px]">{endpoint.retired.replacement}</code> instead.
                </p>
              ) : (
                <p className="text-sm">This command has no canonical equivalent.</p>
              )}
              <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{endpoint.retired.migration}</p>
            </div>
          )}
          <div className="grid gap-6 md:grid-cols-2">
            {/* Parameters */}
            <div className="min-w-0">
              <h4 className={`${CAPTION} mb-2`}>
                PARAMETERS
              </h4>
              {endpoint.params && endpoint.params.length > 0 ? (
                <div>
                  {endpoint.params.map((param) => (
                    <div key={param.name} className="border-t border-border py-2.5">
                      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                        <code className="font-mono text-[13px] [overflow-wrap:anywhere]">{param.name}</code>
                        <span className={CAPTION}>
                          {param.type}
                        </span>
                        {param.required && (
                          <span className={`${CAPTION} text-foreground`}>
                            required
                          </span>
                        )}
                      </div>
                      <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                        {param.description}
                      </p>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="border-t border-border pt-2.5 text-xs text-muted-foreground">No parameters</p>
              )}
            </div>

            {/* Response */}
            <CodeTile
              label="RESPONSE"
              code={endpoint.response}
              report={{ surface: "api_docs", item: "endpoint_response" }}
              codeClassName="text-xs sm:text-xs"
            />
          </div>

          {/* Try it button (a retired route answers 410 to everyone) */}
          {!endpoint.retired && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onTryIt();
              }}
              className={cn(ACTION, "mt-5 min-h-10 cursor-pointer px-5 py-2.5")}
            >
              <Play aria-hidden="true" size={14} />
              Try it
            </button>
          )}
        </div>
      )}
    </div>
  );
}
