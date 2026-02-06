"use client";

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
      <div className="relative w-full max-w-3xl max-h-[90vh] mx-4 bg-card border border-border shadow-lg overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-border bg-muted/30">
          <div className="flex items-center gap-3">
            <span className={`font-mono text-[10px] tracking-wider px-2 py-1 border ${getMethodColor(endpoint.method)}`}>
              {endpoint.method}
            </span>
            <code className="font-mono text-sm">{endpoint.path}</code>
          </div>
          <button
            onClick={onClose}
            className="p-1 hover:bg-muted rounded transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-4 space-y-6">
          {/* Parameters */}
          {(pathParams.length > 0 || queryParams.length > 0) && (
            <div>
              <h4 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-3">
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
              <h4 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-3">
                AUTHORIZATION
              </h4>
              <div className="space-y-2">
                <input
                  type="text"
                  placeholder="Bearer token (JWT or API key)"
                  value={authToken}
                  onChange={(e) => setAuthToken(e.target.value)}
                  className="w-full px-3 py-2 bg-background border border-border font-mono text-sm placeholder:text-muted-foreground focus:outline-none focus:border-foreground transition-colors"
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
              <h4 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                CURL COMMAND
              </h4>
              <button
                onClick={copyCurl}
                className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                {copiedCurl ? <Check size={12} /> : <Copy size={12} />}
                {copiedCurl ? "Copied" : "Copy"}
              </button>
            </div>
            <div className="bg-foreground text-background p-3 overflow-x-auto rounded-sm">
              <pre className="font-mono text-xs leading-relaxed whitespace-pre-wrap">
                <code>{buildCurlCommand()}</code>
              </pre>
            </div>
          </div>

          {/* Response */}
          {(response || error) && (
            <div>
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <h4 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                    RESPONSE
                  </h4>
                  {responseStatus && (
                    <span className={`font-mono text-xs px-2 py-0.5 rounded ${
                      responseStatus >= 200 && responseStatus < 300
                        ? "bg-emerald-500/10 text-emerald-600"
                        : "bg-red-500/10 text-red-600"
                    }`}>
                      {responseStatus}
                    </span>
                  )}
                </div>
                {response && (
                  <button
                    onClick={copyResponse}
                    className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {copiedResponse ? <Check size={12} /> : <Copy size={12} />}
                    {copiedResponse ? "Copied" : "Copy"}
                  </button>
                )}
              </div>
              <div className={`p-3 overflow-x-auto rounded-sm ${error ? "bg-red-500/10 border border-red-500/20" : "bg-foreground text-background"}`}>
                <pre className={`font-mono text-xs leading-relaxed whitespace-pre-wrap ${error ? "text-red-600" : ""}`}>
                  <code>{error || response}</code>
                </pre>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border bg-muted/30">
          <button
            onClick={executeRequest}
            disabled={isLoading}
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-foreground text-background font-mono text-sm hover:bg-foreground/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
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
        <label className="font-mono text-xs">{param.name}</label>
        <span className="font-mono text-[10px] text-muted-foreground">{param.type}</span>
        {param.required && (
          <span className="font-mono text-[10px] text-red-500">required</span>
        )}
        {isPathParam && (
          <span className="font-mono text-[10px] text-blue-500">path</span>
        )}
      </div>
      <input
        type="text"
        placeholder={param.description}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-3 py-2 bg-background border border-border font-mono text-sm placeholder:text-muted-foreground focus:outline-none focus:border-foreground transition-colors"
      />
    </div>
  );
}

// Helper to get method color (duplicated from parent for isolation)
function getMethodColor(method: string) {
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
}
