"use client";

import { useState } from "react";
import { Copy, Check, Terminal, Download, Github } from "lucide-react";

export function SkillInstall() {
  const [copied, setCopied] = useState<string | null>(null);

  const copy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  };

  const installCommand = "curl -sL solvr.dev/install.sh | bash";
  const manualPath = "~/.claude/skills/solvr/";

  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 border-b border-border">
      <div className="max-w-4xl mx-auto">
        <div className="text-center mb-12">
          <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground">
            INSTALLATION
          </span>
          <h2 className="text-2xl md:text-3xl font-light tracking-tight mt-4 mb-4">
            Three ways to install
          </h2>
          <p className="text-muted-foreground max-w-lg mx-auto">
            Choose your preferred method. All install to the same location.
          </p>
        </div>

        <div className="space-y-6">
          {/* Method 1: curl */}
          <div className="border border-border">
            <div className="flex items-center gap-3 px-4 py-3 border-b border-border bg-muted/30">
              <Terminal size={14} className="text-muted-foreground" />
              <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                RECOMMENDED
              </span>
            </div>
            <div className="p-4">
              <h3 className="font-medium text-sm mb-2">One-liner install</h3>
              <div className="bg-foreground text-background p-3 flex items-center justify-between gap-4">
                <code className="font-mono text-xs md:text-sm truncate">
                  {installCommand}
                </code>
                <button
                  onClick={() => copy(installCommand, "curl")}
                  className="shrink-0 hover:opacity-70 transition-opacity"
                >
                  {copied === "curl" ? <Check size={14} /> : <Copy size={14} />}
                </button>
              </div>
              <p className="text-xs text-muted-foreground mt-2">
                Downloads and installs to {manualPath}
              </p>
            </div>
          </div>

          {/* Method 2: ZIP */}
          <div className="border border-border">
            <div className="flex items-center gap-3 px-4 py-3 border-b border-border bg-muted/30">
              <Download size={14} className="text-muted-foreground" />
              <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                MANUAL
              </span>
            </div>
            <div className="p-4">
              <h3 className="font-medium text-sm mb-2">Download ZIP</h3>
              <div className="flex items-center gap-4">
                <a
                  href="/solvr-skill.zip"
                  download
                  className="inline-flex items-center gap-2 px-4 py-2 bg-foreground text-background font-mono text-xs hover:opacity-90 transition-opacity"
                >
                  <Download size={14} />
                  solvr-skill.zip
                </a>
              </div>
              <p className="text-xs text-muted-foreground mt-2">
                Extract to {manualPath}
              </p>
            </div>
          </div>

          {/* Method 3: GitHub */}
          <div className="border border-border">
            <div className="flex items-center gap-3 px-4 py-3 border-b border-border bg-muted/30">
              <Github size={14} className="text-muted-foreground" />
              <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                SOURCE
              </span>
            </div>
            <div className="p-4">
              <h3 className="font-medium text-sm mb-2">Clone from GitHub</h3>
              <div className="bg-foreground text-background p-3 flex items-center justify-between gap-4">
                <code className="font-mono text-xs md:text-sm truncate">
                  git clone https://github.com/fcavalcantirj/solvr.git && cp -r solvr/skill ~/.claude/skills/solvr
                </code>
                <button
                  onClick={() => copy("git clone https://github.com/fcavalcantirj/solvr.git && cp -r solvr/skill ~/.claude/skills/solvr", "git")}
                  className="shrink-0 hover:opacity-70 transition-opacity"
                >
                  {copied === "git" ? <Check size={14} /> : <Copy size={14} />}
                </button>
              </div>
              <p className="text-xs text-muted-foreground mt-2">
                <a
                  href="https://github.com/fcavalcantirj/solvr/tree/main/skill"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-foreground transition-colors underline"
                >
                  View skill folder on GitHub
                </a>
              </p>
            </div>
          </div>
        </div>

        {/* What gets installed */}
        <div className="mt-12 border border-border p-6">
          <h3 className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-4">
            INSTALLED FILES
          </h3>
          <div className="font-mono text-sm text-muted-foreground space-y-1">
            <div>~/.claude/skills/solvr/</div>
            <div className="pl-4">├── SKILL.md</div>
            <div className="pl-4">├── skill.json</div>
            <div className="pl-4">├── references/</div>
            <div className="pl-8">├── api.md</div>
            <div className="pl-8">└── examples.md</div>
            <div className="pl-4">├── scripts/</div>
            <div className="pl-8">└── solvr.sh</div>
            <div className="pl-4">└── LICENSE</div>
          </div>
        </div>
      </div>
    </section>
  );
}
