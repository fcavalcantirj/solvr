"use client";

import { CAPTION } from "@/components/page/caption";
import { ACTION, FOCUS } from "@/components/page/marketing";
import { cn } from "@/lib/utils";

import { useState, useCallback } from "react";
import { X, Copy, Check, Play, Loader2 } from "lucide-react";
import { Endpoint, Param } from "./api-endpoint-types";

interface ApiPlaygroundProps {
  endpoint: Endpoint;
  isOpen: boolean;
  onClose: () => void;
}

interface ParamValues {
  [key: string]: string;
}

const BASE_URL = "https://api.solvr.dev/v1";

const INPUT =
  "w-full border border-border bg-background px-3 py-2 font-mono text-sm transition-colors placeholder:text-muted-foreground focus-visible:border-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground";
const COPY =
  "flex min-h-8 cursor-pointer items-center gap-1.5 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-foreground";

export function ApiPlayground({ endpoint, isOpen, onClose }: ApiPlaygroundProps) {
  const [paramValues, setParamValues] = useState<ParamValues>({});
  const [authToken, setAuthToken] = useState("");
  const [response, setResponse] = useState<string | null>(null);
  const [responseStatus, setResponseStatus] = useState<number | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copiedCurl, setCopiedCurl] = useState(false);
  const [copiedResponse, setCopiedResponse] = useState(false);

  // Parse path params from endpoint path (e.g., /posts/{id} -> ["id"])
  const pathParams = endpoint.path.match(/\{([^}]+)\}/g)?.map(p => p.slice(1, -1)) || [];

  // Get query params (params that aren't path params)
  const queryParams = endpoint.params?.filter(p => !pathParams.includes(p.name)) || [];

  // Build the actual URL with path params replaced
  const buildUrl = useCallback(() => {
    let url = endpoint.path;
    pathParams.forEach(param => {
      url = url.replace(`{${param}}`, paramValues[param] || `{${param}}`);
    });

    // Add query params
    const queryString = queryParams
      .filter(p => paramValues[p.name])
      .map(p => `${encodeURIComponent(p.name)}=${encodeURIComponent(paramValues[p.name])}`)
      .join("&");

    if (queryString) {
      url += `?${queryString}`;
    }

    return `${BASE_URL}${url}`;
  }, [endpoint.path, pathParams, queryParams, paramValues]);

  // Build curl command
  const buildCurlCommand = useCallback(() => {
    const url = buildUrl();
    let curl = `curl -X ${endpoint.method} "${url}"`;

    if (authToken) {
      curl += ` \\\n  -H "Authorization: Bearer ${authToken}"`;
    }

    if (endpoint.method === "POST" || endpoint.method === "PATCH") {
      curl += ` \\\n  -H "Content-Type: application/json"`;
      // For POST/PATCH, we'd need body params - simplified for now
      const bodyParams = endpoint.params?.filter(p =>
        !pathParams.includes(p.name) &&
        !["limit", "offset", "sort", "order", "q", "type", "tags", "status"].includes(p.name)
      ) || [];
      if (bodyParams.length > 0) {
        const body: Record<string, string> = {};
        bodyParams.forEach(p => {
          if (paramValues[p.name]) {
            body[p.name] = paramValues[p.name];
          }
        });
        if (Object.keys(body).length > 0) {
          curl += ` \\\n  -d '${JSON.stringify(body)}'`;
        }
      }
    }

    return curl;
  }, [buildUrl, endpoint.method, endpoint.params, authToken, pathParams, paramValues]);

  // Copy to clipboard helpers
  const copyCurl = () => {
    navigator.clipboard.writeText(buildCurlCommand());
    setCopiedCurl(true);
    setTimeout(() => setCopiedCurl(false), 2000);
  };

  const copyResponse = () => {
    if (response) {
      navigator.clipboard.writeText(response);
      setCopiedResponse(true);
      setTimeout(() => setCopiedResponse(false), 2000);
    }
  };

  // Execute the API call
  const executeRequest = async () => {
    setIsLoading(true);
    setError(null);
    setResponse(null);
    setResponseStatus(null);

    try {
      const url = buildUrl();
      const headers: HeadersInit = {};

      if (authToken) {
        headers["Authorization"] = `Bearer ${authToken}`;
      }

      if (endpoint.method === "POST" || endpoint.method === "PATCH") {
        headers["Content-Type"] = "application/json";
      }

      const options: RequestInit = {
        method: endpoint.method,
        headers,
      };

      // Add body for POST/PATCH
      if (endpoint.method === "POST" || endpoint.method === "PATCH") {
        const bodyParams = endpoint.params?.filter(p =>
          !pathParams.includes(p.name) &&
          !["limit", "offset", "sort", "order", "q", "type", "tags", "status"].includes(p.name)
        ) || [];
        const body: Record<string, string> = {};
        bodyParams.forEach(p => {
          if (paramValues[p.name]) {
            body[p.name] = paramValues[p.name];
          }
        });
        if (Object.keys(body).length > 0) {
          options.body = JSON.stringify(body);
        }
      }

      const res = await fetch(url, options);
      setResponseStatus(res.status);

      const text = await res.text();
      try {
        const json = JSON.parse(text);
        setResponse(JSON.stringify(json, null, 2));
      } catch {
        setResponse(text);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Request failed");
    } finally {
      setIsLoading(false);
    }
  };

  // Handle param change
  const handleParamChange = (name: string, value: string) => {
    setParamValues(prev => ({ ...prev, [name]: value }));
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative mx-4 flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden border border-foreground bg-background">
        {/* Header */}
        <div className="flex items-center justify-between gap-4 border-b border-border py-3 pl-5 pr-3">
          <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
            <span className="font-mono text-[11px] uppercase tracking-[0.18em]">
              {endpoint.method}
            </span>
            <code className="min-w-0 font-mono text-sm [overflow-wrap:anywhere]">{endpoint.path}</code>
          </div>
          <button
            type="button"
            onClick={onClose}
            className={cn("shrink-0 cursor-pointer p-2 transition-colors hover:text-muted-foreground", FOCUS)}
          >
            <X size={18} />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 space-y-6 overflow-y-auto p-5">
          {/* Parameters */}
          {(pathParams.length > 0 || queryParams.length > 0) && (
            <div>
              <h4 className={`${CAPTION} mb-3`}>
                PARAMETERS
              </h4>
              <div className="space-y-3">
                {/* Path params first */}
                {pathParams.map(paramName => {
                  const paramDef = endpoint.params?.find(p => p.name === paramName);
                  return (
                    <ParamInput
                      key={paramName}
                      param={{
                        name: paramName,
                        type: paramDef?.type || "string",
                        required: true,
                        description: paramDef?.description || `Path parameter: ${paramName}`
                      }}
                      value={paramValues[paramName] || ""}
                      onChange={(v) => handleParamChange(paramName, v)}
                      isPathParam
                    />
                  );
                })}
                {/* Query params */}
                {queryParams.map(param => (
                  <ParamInput
                    key={param.name}
                    param={param}
                    value={paramValues[param.name] || ""}
                    onChange={(v) => handleParamChange(param.name, v)}
                  />
                ))}
              </div>
            </div>
          )}

          {/* Auth Token */}
          {endpoint.auth && endpoint.auth !== "none" && (
            <div>
              <h4 className={`${CAPTION} mb-3`}>
                AUTHORIZATION
              </h4>
              <div className="space-y-2">
                <input
                  type="text"
                  placeholder="Bearer token (JWT or API key)"
                  value={authToken}
                  onChange={(e) => setAuthToken(e.target.value)}
                  className={INPUT}
                />
                <p className="text-xs text-muted-foreground">
                  Required: {endpoint.auth === "jwt" ? "JWT token" : endpoint.auth === "api_key" ? "API key" : "JWT or API key"}
                </p>
              </div>
            </div>
          )}

          {/* Curl Command */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <h4 className={CAPTION}>
                CURL COMMAND
              </h4>
              <button
                type="button"
                onClick={copyCurl}
                className={cn(COPY, FOCUS)}
              >
                {copiedCurl ? <Check size={12} /> : <Copy size={12} />}
                {copiedCurl ? "Copied" : "Copy"}
              </button>
            </div>
            <div className="min-w-0 bg-foreground p-4 text-background">
              <pre className="whitespace-pre-wrap font-mono text-xs leading-relaxed [overflow-wrap:anywhere]">
                <code>{buildCurlCommand()}</code>
              </pre>
            </div>
          </div>

          {/* Response */}
          {(response || error) && (
            <div>
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <h4 className={CAPTION}>
                    RESPONSE
                  </h4>
                  {responseStatus && (
                    <span className="bg-secondary px-2 py-0.5 font-mono text-xs text-foreground">
                      {responseStatus}
                    </span>
                  )}
                </div>
                {response && (
                  <button
                    type="button"
                    onClick={copyResponse}
                    className={cn(COPY, FOCUS)}
                  >
                    {copiedResponse ? <Check size={12} /> : <Copy size={12} />}
                    {copiedResponse ? "Copied" : "Copy"}
                  </button>
                )}
              </div>
              <div className={`min-w-0 p-4 ${error ? "border-l-2 border-foreground bg-secondary" : "bg-foreground text-background"}`}>
                <pre className={`whitespace-pre-wrap font-mono text-xs leading-relaxed [overflow-wrap:anywhere] ${error ? "text-foreground" : ""}`}>
                  <code>{error || response}</code>
                </pre>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="border-t border-border p-4">
          <button
            type="button"
            onClick={executeRequest}
            disabled={isLoading}
            className={cn(ACTION, "w-full cursor-pointer disabled:cursor-not-allowed disabled:opacity-50")}
          >
            {isLoading ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Sending...
              </>
            ) : (
              <>
                <Play size={16} />
                Send Request
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

// Helper component for parameter inputs
interface ParamInputProps {
  param: Param;
  value: string;
  onChange: (value: string) => void;
  isPathParam?: boolean;
}

function ParamInput({ param, value, onChange, isPathParam }: ParamInputProps) {
  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <label className="font-mono text-[13px]">{param.name}</label>
        <span className={CAPTION}>{param.type}</span>
        {param.required && (
          <span className={`${CAPTION} text-foreground`}>required</span>
        )}
        {isPathParam && (
          <span className={`${CAPTION} text-foreground`}>path</span>
        )}
      </div>
      <input
        type="text"
        placeholder={param.description}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={INPUT}
      />
    </div>
  );
}
