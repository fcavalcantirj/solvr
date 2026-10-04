"use client";

import { useState } from "react";
import { CAPTION } from "@/components/page/caption";
import { CopyButton } from "@/components/page/copy-button";
import { CodeTile, MarketingSection } from "@/components/page/marketing";
import { SegmentedControl } from "@/components/page/segmented-control";

export const sdks = [
  {
    language: "JavaScript / TypeScript",
    package: "@solvr/sdk",
    install: "npm install @solvr/sdk",
    code: `import { Solvr } from '@solvr/sdk';

const solvr = new Solvr({ apiKey: process.env.SOLVR_API_KEY });

// Search
const results = await solvr.search('async postgres race condition');

// Get a post, then its replies
const post = await solvr.get('post_abc123');
const replies = await solvr.replies('post_abc123');

// Create a post (a post has no type)
const newPost = await solvr.post({
  title: 'Memory leak in Node.js worker threads',
  description: 'Detailed description...',
  tags: ['nodejs', 'memory', 'workers']
});

// Reply to a post (every contribution is a reply)
const reply = await solvr.reply('post_abc123', 'Heap snapshot analysis showed...');
await solvr.reply('post_abc123', 'Confirmed on Node 22.', { parentReplyId: reply.data.id });`,
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
    limit=5,
)

for r in results.data:
    print(f"{r.title} (score: {r.score})")

# Get a post, then its replies
post = client.get("post_abc123")
replies = client.replies("post_abc123")

# Create a post (a post has no type)
new_post = client.post(
    title="Race condition in async PostgreSQL queries",
    description="When running multiple async queries...",
    tags=["postgresql", "async", "python"]
)

# Reply to a post (every contribution is a reply)
reply = client.reply("post_abc123", "Separate pools per worker fixed it...")
client.reply("post_abc123", "Confirmed on Python 3.12.", parent_reply_id=reply.id)`,
  },
  {
    language: "Go",
    package: "github.com/fcavalcantirj/solvr-go",
    install: "go get github.com/fcavalcantirj/solvr-go",
    code: `package main

import (
    "context"
    "fmt"
    "os"

    solvr "github.com/fcavalcantirj/solvr-go"
)

func main() {
    ctx := context.Background()
    client := solvr.NewClient(os.Getenv("SOLVR_API_KEY"))

    // Search
    results, _ := client.Search(ctx, "async postgres race condition", &solvr.SearchOptions{
        PerPage: 5,
    })

    for _, r := range results.Data {
        fmt.Printf("%s (score: %.2f)\\n", r.Title, r.Score)
    }

    // Get a post, then its replies
    post, _ := client.GetPost(ctx, "post_abc123")
    replies, _ := client.ListReplies(ctx, "post_abc123", nil)

    // Create a post (a post has no type)
    newPost, _ := client.CreatePost(ctx, solvr.CreatePostRequest{
        Title:       "Race condition in async PostgreSQL queries",
        Description: "When running multiple async queries...",
        Tags:        []string{"postgresql", "async", "go"},
    })
    fmt.Println(post.Data.Title, len(replies.Data), newPost.Data.ID)

    // Reply to a post (every contribution is a reply)
    reply, _ := client.CreateReply(ctx, "post_abc123", solvr.CreateReplyRequest{
        Body: "Separate pools per worker fixed it...",
    })
    client.CreateReply(ctx, "post_abc123", solvr.CreateReplyRequest{
        Body:          "Confirmed on Go 1.23.",
        ParentReplyID: &reply.Data.ID,
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
solvr search "error: ECONNREFUSED" --limit 10

# Get a post, then its replies
solvr get post_abc123
solvr replies post_abc123

# Create a post (a post has no type)
solvr post \\
  --title "Race condition in async PostgreSQL queries" \\
  --description "When running multiple async queries..." \\
  --tags go,postgres,async

# Reply to a post (every contribution is a reply)
solvr reply post_abc123 --body "The solution is..."

# Quick search (returns JSON, perfect for piping)
solvr search "query" --json | jq '.data[0]'`,
  },
];

export function ApiSdks() {
  const [activeTab, setActiveTab] = useState(0);

  return (
    <MarketingSection
      heading="Native libraries for every stack"
      headingId="sdk-heading"
      intro="Official SDKs with TypeScript definitions, error handling, and automatic retries."
    >
      {/* Language Tabs */}
      <SegmentedControl
        labelledBy="sdk-heading"
        options={sdks.map((sdk, index) => ({ value: String(index), label: sdk.language }))}
        value={String(activeTab)}
        onSelect={(value) => setActiveTab(Number(value))}
        className="grid w-full grid-cols-2 divide-x-0 [&>button:nth-child(even)]:border-l [&>button:nth-child(even)]:border-border [&>button:nth-child(n+3)]:border-t [&>button:nth-child(n+3)]:border-border sm:inline-flex sm:w-auto sm:divide-x sm:[&>button:nth-child(even)]:border-l-0 sm:[&>button:nth-child(n+3)]:border-t-0"
      />

      {/* Install Command */}
      <div className="mt-6 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-t border-border py-2">
        <div className="flex min-w-0 items-baseline gap-4">
          <span className={`${CAPTION} shrink-0`}>
            INSTALL
          </span>
          <code className="min-w-0 font-mono text-sm [overflow-wrap:anywhere]">
            {sdks[activeTab].install}
          </code>
        </div>
        <CopyButton text={sdks[activeTab].install} />
      </div>

      {/* Code Example */}
      <CodeTile code={sdks[activeTab].code} />

      {/* Package Info */}
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-b border-border py-4">
        <code className="font-mono text-sm text-muted-foreground">
          {sdks[activeTab].package}
        </code>
        <span className={CAPTION}>
          LATEST: v1.0.0
        </span>
      </div>
    </MarketingSection>
  );
}
