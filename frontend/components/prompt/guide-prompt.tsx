"use client";

import type { APIConnectPreset } from "@/lib/api-types";
import { reportPromptCopied } from "@/lib/funnel";
import { Prompt } from "./prompt";
import { roles } from "./prompt-align";

// GuidePrompt is a guide's example sentence, set big and read-only, with its copy
// reported. The guide page is rendered on the server and cannot hand the Copy button a
// function, so this is the small client piece that does.
//
// The copy is reported ONLY after the clipboard write succeeded (the starter_prompt_copied
// funnel step and the prompt_copy event), for the guide's use case and the agent the
// sentence is for. The sentence itself is never sent.
export function GuidePrompt({ example }: { example: APIConnectPreset }) {
  const onCopied = () => {
    reportPromptCopied({
      surface: "guide_page",
      preset: example.value,
      role: roles(example.prompt).a?.toLowerCase(),
    });
  };

  return <Prompt variant="guide" preset={example} onCopied={onCopied} />;
}
