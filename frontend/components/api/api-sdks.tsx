"use client";

import { useState } from "react";
import { Copy, Check } from "lucide-react";

const sdks = [
  {
    language: "JavaScript / TypeScript",
    package: "@solvr/sdk",
    install: "npm install @solvr/sdk",
    code: `import { Solvr } from '@solvr/sdk';

const solvr = new Solvr({ apiKey: process.env.SOLVR_API_KEY });

// Search
const results = await solvr.search('async postgres race condition');

// Get post details
const post = await solvr.get('post_abc123', { 
  include: ['approaches', 'answers'] 
});

// Create a problem
const newPost = await solvr.post({
  type: 'problem',
  title: 'Memory leak in Node.js worker threads',
  description: 'Detailed description...',
  tags: ['nodejs', 'memory', 'workers']
});

// Add an approach
await solvr.approach('post_abc123', {
  angle: 'Heap snapshot analysis',
  method: 'Using Chrome DevTools...'
});`,
  },
  {
    language: "Python",
    package: "solvr",
    install: "pip install solvr",
    code: `from solvr import Solvr
import os

client = Solvr(api_key=os.environ['SOLVR_API_KEY'])

# Search
results = client.search(
    "async postgres race condition", 
    type="problem",
    limit=5
)

for r in results:
    print(f"{r.title} (score: {r.score})")

# Get post details
post = client.get("post_abc123", include=["approaches", "answers"])

# Create a problem
new_post = client.post(
    type="problem",
    title="Race condition in async PostgreSQL queries",
    description="When running multiple async queries...",
    tags=["postgresql", "async", "python"]
)

# Add an approach
client.approach("post_abc123", 
    angle="Connection pool isolation",
    method="Separate pools per worker..."
)`,
  },
  {
    language: "Go",
    package: "github.com/fcavalcantirj/solvr-go",
    install: "go get github.com/fcavalcantirj/solvr-go",
    code: `package main

import (
    "fmt"
    "os"
    
    solvr "github.com/fcavalcantirj/solvr-go"
)

func main() {
    client := solvr.New(os.Getenv("SOLVR_API_KEY"))
    
    // Search
    results, _ := client.Search("async postgres race condition", solvr.SearchOpts{
        Type:  "problem",
        Limit: 5,
    })
    
    for _, r := range results {
        fmt.Printf("%s (score: %.2f)\\n", r.Title, r.Score)
    }
    
    // Get post details
    post, _ := client.Get("post_abc123", solvr.GetOpts{
        Include: []string{"approaches", "answers"},
    })
    
    // Create a problem
    newPost, _ := client.Post(solvr.Post{
        Type:        "problem",
        Title:       "Race condition in async PostgreSQL queries",
        Description: "When running multiple async queries...",
        Tags:        []string{"postgresql", "async", "go"},
    })
}`,
  },
  {
    language: "CLI",
    package: "@solvr/cli",
    install: "npm install -g @solvr/cli",
    code: `# Configure
solvr config set api-key solvr_sk_xxxxx

# Search
solvr search "async postgres race condition"
solvr search "error: ECONNREFUSED" --type problem --limit 10

# Get post details
solvr get post_abc123 --include approaches,answers

# Create a problem
solvr post problem \\
  --title "Race condition in async PostgreSQL queries" \\
  --description "When running multiple async queries..." \\
  --tags go,postgres,async

# Add an answer
solvr answer post_abc123 --content "The solution is..."

# Quick search (returns JSON, perfect for piping)
solvr search "query" --json | jq '.data[0]'`,
  },
];

export function ApiSdks() {
  const [activeTab, setActiveTab] = useState(0);
  const [copied, setCopied] = useState<string | null>(null);

  const copy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  };

  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 border-b border-border">
      <div className="max-w-7xl mx-auto">
        <div className="mb-12 lg:mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            SDKS & CLI
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Native libraries for every stack
          </h2>
          <p className="text-muted-foreground max-w-2xl">
            Official SDKs with TypeScript definitions, error handling, and automatic retries.
          </p>
        </div>

        {/* Language Tabs */}
        <div className="flex flex-wrap gap-2 mb-6 border-b border-border pb-6">
          {sdks.map((sdk, index) => (
            <button
              key={sdk.language}
              onClick={() => setActiveTab(index)}
              className={`font-mono text-xs tracking-wider px-4 py-2 transition-colors ${
                activeTab === index
                  ? "bg-foreground text-background"
                  : "border border-border hover:bg-muted"
              }`}
            >
              {sdk.language}
            </button>
          ))}
        </div>

        {/* Active SDK */}
        <div className="border border-border">
          {/* Install Command */}
          <div className="flex items-center justify-between gap-4 px-4 lg:px-6 py-4 border-b border-border bg-muted/30">
            <div className="flex items-center gap-4 min-w-0">
              <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground shrink-0">
                INSTALL
              </span>
              <code className="font-mono text-sm truncate">
                {sdks[activeTab].install}
              </code>
            </div>
            <button
              onClick={() => copy(sdks[activeTab].install, "install")}
              className="shrink-0 hover:text-muted-foreground transition-colors"
            >
              {copied === "install" ? <Check size={14} /> : <Copy size={14} />}
            </button>
          </div>

          {/* Code Example */}
          <div className="relative group">
            <button
              onClick={() => copy(sdks[activeTab].code, "code")}
              className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity hover:text-background/70 z-10"
            >
              {copied === "code" ? (
                <Check size={14} className="text-background" />
              ) : (
                <Copy size={14} className="text-background" />
              )}
            </button>
            <div className="bg-foreground text-background p-6 overflow-x-auto">
              <pre className="font-mono text-xs md:text-sm leading-relaxed">
                <code>{sdks[activeTab].code}</code>
              </pre>
            </div>
          </div>

          {/* Package Info */}
          <div className="flex items-center justify-between px-4 lg:px-6 py-4 border-t border-border bg-muted/30">
            <code className="font-mono text-sm text-muted-foreground">
              {sdks[activeTab].package}
            </code>
            <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
              LATEST: v1.0.0
            </span>
          </div>
        </div>
      </div>
    </section>
  );
}
