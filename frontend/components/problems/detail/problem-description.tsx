"use client";

import { CheckCircle, Circle, Code2 } from "lucide-react";

const tags = ["node.js", "postgresql", "async", "concurrency", "connection-pool"];

const successCriteria = [
  { text: "Connection pool maintains stable size under 500+ concurrent requests", verified: false },
  { text: "No timeout errors after 1 hour of sustained load", verified: false },
  { text: "Solution works with pg-pool library", verified: false },
];

const codeSnippet = `// Current problematic code
const pool = new Pool({ max: 20 });

async function handleRequest(req, res) {
  const client = await pool.connect();
  try {
    const result = await client.query('SELECT * FROM users WHERE id = $1', [req.params.id]);
    // Sometimes connection is released before response is sent
    await someAsyncOperation(result);
    res.json(result.rows);
  } finally {
    client.release(); // Race condition here
  }
}`;

export function ProblemDescription() {
  return (
    <div className="space-y-6">
      {/* Description */}
      <div className="prose prose-sm max-w-none">
        <p className="text-foreground/90 leading-relaxed">
          Multiple concurrent requests causing connection release timing issues. Under load testing 
          with 100+ concurrent users, connections are not being properly returned to the pool, 
          leading to pool exhaustion and timeout errors.
        </p>
        <p className="text-foreground/90 leading-relaxed">
          Tried <code className="font-mono text-xs bg-secondary px-1.5 py-0.5">Promise.all()</code> and 
          sequential awaits but the race condition persists. The issue seems to occur specifically 
          when the async operation between query and response takes variable time.
        </p>
        <p className="text-foreground/90 leading-relaxed">
          Environment: Node.js 20.x, pg-pool 3.6.x, PostgreSQL 15. Running on AWS with connection 
          pooling via PgBouncer, but the issue reproduces even without PgBouncer.
        </p>
      </div>

      {/* Code Snippet */}
      <div className="border border-border bg-foreground text-background">
        <div className="px-4 py-2 border-b border-background/20 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Code2 size={14} />
            <span className="font-mono text-xs tracking-wider">PROBLEMATIC CODE</span>
          </div>
          <span className="font-mono text-[10px] tracking-wider text-background/50">
            server.js
          </span>
        </div>
        <pre className="p-4 overflow-x-auto text-sm">
          <code className="font-mono text-xs leading-relaxed">{codeSnippet}</code>
        </pre>
      </div>

      {/* Success Criteria */}
      <div className="border border-border bg-card">
        <div className="px-4 py-3 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">SUCCESS CRITERIA</h3>
        </div>
        <div className="p-4 space-y-3">
          {successCriteria.map((criteria, index) => (
            <div key={index} className="flex items-start gap-3">
              {criteria.verified ? (
                <CheckCircle size={16} className="text-foreground mt-0.5 flex-shrink-0" />
              ) : (
                <Circle size={16} className="text-muted-foreground mt-0.5 flex-shrink-0" />
              )}
              <span className={`text-sm ${criteria.verified ? "text-foreground" : "text-foreground/80"}`}>
                {criteria.text}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Tags */}
      <div className="flex flex-wrap gap-2">
        {tags.map((tag) => (
          <span
            key={tag}
            className="font-mono text-[10px] tracking-wider text-muted-foreground bg-secondary px-3 py-1.5 hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
          >
            {tag}
          </span>
        ))}
      </div>
    </div>
  );
}
