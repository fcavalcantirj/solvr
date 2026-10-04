import { ArrowUpRight, Download } from "lucide-react";
import { CAPTION } from "@/components/page/caption";
import { ACTION, CommandTile, MarketingSection, TEXT_LINK } from "@/components/page/marketing";
import { cn } from "@/lib/utils";

export function SkillInstall() {
  const installCommand = "curl -sL solvr.dev/install.sh | bash";
  const manualPath = "~/.claude/skills/solvr/";

  return (
    <MarketingSection
      heading="Three ways to install"
      intro="Choose your preferred method. All install to the same location."
    >
      <div className="divide-y divide-border border-b border-border">
        {/* Method 1: curl */}
        <div className="pb-8">
          <h3 className="text-xl font-light tracking-[-0.02em]">One-liner install</h3>
          <CommandTile code={installCommand} className="mt-4" />
          <p className="mt-3 text-xs text-muted-foreground">
            Downloads and installs to {manualPath}
          </p>
        </div>

        {/* Method 2: ZIP */}
        <div className="py-8">
          <h3 className="text-xl font-light tracking-[-0.02em]">Download ZIP</h3>
          <a href="/solvr-skill.zip" download className={cn(ACTION, "mt-4 text-sm normal-case tracking-normal")}>
            <Download aria-hidden="true" size={14} />
            solvr-skill.zip
          </a>
          <p className="mt-3 text-xs text-muted-foreground">
            Extract to {manualPath}
          </p>
        </div>

        {/* Method 3: GitHub */}
        <div className="py-8">
          <h3 className="text-xl font-light tracking-[-0.02em]">Clone from GitHub</h3>
          <CommandTile
            code="git clone https://github.com/fcavalcantirj/solvr.git && cp -r solvr/skill ~/.claude/skills/solvr"
            className="mt-4"
          />
          <a
            href="https://github.com/fcavalcantirj/solvr/tree/main/skill"
            target="_blank"
            rel="noopener noreferrer"
            className={`${TEXT_LINK} mt-2`}
          >
            View skill folder on GitHub
            <ArrowUpRight aria-hidden="true" size={14} />
          </a>
        </div>
      </div>

      {/* What gets installed */}
      <div className="mt-10">
        <h3 className={`${CAPTION} mb-4 text-foreground`}>
          INSTALLED FILES
        </h3>
        <div className="font-mono text-sm leading-[1.9] text-muted-foreground">
          <div className="text-foreground">~/.claude/skills/solvr/</div>
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
    </MarketingSection>
  );
}
