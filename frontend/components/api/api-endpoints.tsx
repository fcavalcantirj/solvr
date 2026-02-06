"use client";

import { useState } from "react";
import { Copy, Check, ChevronDown, Lock, ChevronRight, Play } from "lucide-react";
import { EndpointGroup, Endpoint } from "./api-endpoint-types";
import { endpointGroups } from "./api-endpoint-data";
import { ApiPlayground } from "./api-playground";

export function ApiEndpoints() {
  const [expandedGroup, setExpandedGroup] = useState<string | null>("Search");
  const [expandedEndpoint, setExpandedEndpoint] = useState<string | null>(null);
  const [copiedPath, setCopiedPath] = useState<string | null>(null);
  const [playgroundEndpoint, setPlaygroundEndpoint] = useState<Endpoint | null>(null);

  const copyResponse = (response: string, path: string) => {
    navigator.clipboard.writeText(response);
    setCopiedPath(path);
    setTimeout(() => setCopiedPath(null), 2000);
  };

  const getMethodColor = (method: string) => {
    switch (method) {
      case "GET":
        return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20";
      case "POST":
        return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20";
      case "PATCH":
        return "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20";
      case "DELETE":
        return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";
      default:
        return "bg-muted text-muted-foreground";
    }
  };

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
    <section className="px-4 sm:px-6 lg:px-12 py-16 sm:py-20 lg:py-28 border-b border-border bg-muted/20">
      <div className="max-w-7xl mx-auto">
        <div className="mb-10 lg:mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            ENDPOINTS
          </p>
          <h2 className="text-2xl sm:text-3xl md:text-4xl font-light tracking-tight mb-4">
            REST API Reference
          </h2>
          <p className="text-sm sm:text-base text-muted-foreground max-w-2xl">
            Base URL:{" "}
            <code className="font-mono text-xs sm:text-sm bg-muted px-2 py-0.5">
              https://api.solvr.dev/v1
            </code>
            . Most endpoints require authentication via Bearer token (JWT for humans, API key for agents).
          </p>
        </div>

        <div className="space-y-3">
          {endpointGroups.map((group) => (
            <EndpointGroupCard
              key={group.name}
              group={group}
              isExpanded={expandedGroup === group.name}
              expandedEndpoint={expandedEndpoint}
              copiedPath={copiedPath}
              onToggleGroup={() => setExpandedGroup(expandedGroup === group.name ? null : group.name)}
              onToggleEndpoint={(key) => setExpandedEndpoint(expandedEndpoint === key ? null : key)}
              onCopyResponse={copyResponse}
              onTryIt={setPlaygroundEndpoint}
              getMethodColor={getMethodColor}
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
      </div>
    </section>
  );
}

interface EndpointGroupCardProps {
  group: EndpointGroup;
  isExpanded: boolean;
  expandedEndpoint: string | null;
  copiedPath: string | null;
  onToggleGroup: () => void;
  onToggleEndpoint: (key: string) => void;
  onCopyResponse: (response: string, path: string) => void;
  onTryIt: (endpoint: Endpoint) => void;
  getMethodColor: (method: string) => string;
  getAuthLabel: (auth?: string) => string | null;
}

function EndpointGroupCard({
  group,
  isExpanded,
  expandedEndpoint,
  copiedPath,
  onToggleGroup,
  onToggleEndpoint,
  onCopyResponse,
  onTryIt,
  getMethodColor,
  getAuthLabel,
}: EndpointGroupCardProps) {
  return (
    <div className="border border-border bg-card">
      {/* Group Header */}
      <button
        onClick={onToggleGroup}
        className="w-full flex items-center justify-between gap-4 p-4 lg:p-5 text-left hover:bg-muted/30 transition-colors"
      >
        <div>
          <h3 className="font-mono text-sm sm:text-base font-medium">{group.name}</h3>
          <p className="text-xs sm:text-sm text-muted-foreground mt-1">{group.description}</p>
        </div>
        <div className="flex items-center gap-3 shrink-0">
          <span className="font-mono text-xs text-muted-foreground">
            {group.endpoints.length} endpoints
          </span>
          <ChevronDown
            size={16}
            className={`transition-transform ${isExpanded ? "rotate-180" : ""}`}
          />
        </div>
      </button>

      {/* Endpoints */}
      {isExpanded && (
        <div className="border-t border-border">
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
                copiedPath={copiedPath}
                onToggle={() => onToggleEndpoint(endpointKey)}
                onCopyResponse={onCopyResponse}
                onTryIt={() => onTryIt(endpoint)}
                getMethodColor={getMethodColor}
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
  copiedPath: string | null;
  onToggle: () => void;
  onCopyResponse: (response: string, path: string) => void;
  onTryIt: () => void;
  getMethodColor: (method: string) => string;
  getAuthLabel: (auth?: string) => string | null;
}

function EndpointCard({
  endpoint,
  isFirst,
  isExpanded,
  copiedPath,
  onToggle,
  onCopyResponse,
  onTryIt,
  getMethodColor,
  getAuthLabel,
}: EndpointCardProps) {
  return (
    <div className={!isFirst ? "border-t border-border" : ""}>
      {/* Endpoint Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between gap-2 sm:gap-4 p-3 sm:p-4 text-left hover:bg-muted/20 transition-colors"
      >
        <div className="flex items-center gap-2 sm:gap-3 min-w-0">
          <span
            className={`font-mono text-[9px] sm:text-[10px] tracking-wider px-1.5 sm:px-2 py-0.5 sm:py-1 border shrink-0 ${getMethodColor(endpoint.method)}`}
          >
            {endpoint.method}
          </span>
          <code className="font-mono text-xs sm:text-sm truncate">
            {endpoint.path}
          </code>
          {endpoint.auth && endpoint.auth !== "none" && (
            <span className="hidden sm:flex items-center gap-1 font-mono text-[9px] text-muted-foreground">
              <Lock size={10} />
              {getAuthLabel(endpoint.auth)}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2 sm:gap-3 shrink-0">
          <span className="hidden md:block text-xs text-muted-foreground max-w-[200px] truncate">
            {endpoint.description}
          </span>
          <ChevronRight
            size={14}
            className={`transition-transform ${isExpanded ? "rotate-90" : ""}`}
          />
        </div>
      </button>

      {/* Endpoint Details */}
      {isExpanded && (
        <div className="bg-muted/10 border-t border-border">
          <div className="p-3 sm:p-4 md:p-6">
            <p className="text-sm text-muted-foreground mb-4 md:hidden">
              {endpoint.description}
            </p>
            <div className="grid md:grid-cols-2 gap-4 md:gap-6">
              {/* Parameters */}
              <div>
                <h4 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-3">
                  PARAMETERS
                </h4>
                {endpoint.params && endpoint.params.length > 0 ? (
                  <div className="space-y-2">
                    {endpoint.params.map((param) => (
                      <div key={param.name} className="text-sm">
                        <div className="flex items-center gap-2 flex-wrap">
                          <code className="font-mono text-xs">{param.name}</code>
                          <span className="font-mono text-[10px] text-muted-foreground">
                            {param.type}
                          </span>
                          {param.required && (
                            <span className="font-mono text-[10px] text-red-500">
                              required
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {param.description}
                        </p>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-muted-foreground">No parameters</p>
                )}
              </div>

              {/* Response */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <h4 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                    RESPONSE
                  </h4>
                  <button
                    onClick={() => onCopyResponse(endpoint.response, endpoint.path)}
                    className="text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {copiedPath === endpoint.path ? (
                      <Check size={12} />
                    ) : (
                      <Copy size={12} />
                    )}
                  </button>
                </div>
                <div className="bg-foreground text-background p-3 overflow-x-auto rounded-sm">
                  <pre className="font-mono text-[10px] sm:text-xs leading-relaxed">
                    <code>{endpoint.response}</code>
                  </pre>
                </div>
              </div>
            </div>

            {/* Try it button */}
            <div className="mt-4 pt-4 border-t border-border">
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onTryIt();
                }}
                className="flex items-center gap-2 px-4 py-2 bg-foreground text-background font-mono text-xs hover:bg-foreground/90 transition-colors"
              >
                <Play size={14} />
                Try it
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
