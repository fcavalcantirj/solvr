"use client";

import { api } from "@/lib/api";
import type { APIConnectPreset } from "@/lib/api-types";
import { trackCta } from "@/lib/track-attrs";
import { Prompt } from "./prompt";
import { roles } from "./prompt-align";

// GuidePrompt is a guide's example sentence, set big and read-only, with its copy
// reported. The guide page is rendered on the server and cannot hand the Copy button a
// function, so this is the small client piece that does.
//
// starter_prompt_copied is reported ONLY after the clipboard write succeeded, for the
// guide's use case and the agent the sentence is for. The sentence itself is never sent.
export function GuidePrompt({ example }: { example: APIConnectPreset }) {
  const onCopied = () => {
    void api.postFunnelEvent?.({
      event: "starter_prompt_copied",
      entry_surface: "guide_page",
      preset: example.value,
      role: roles(example.prompt).a?.toLowerCase(),
    });
  };

  return <Prompt variant="guide" preset={example} onCopied={onCopied} copyTrack={trackCta("copy_prompt", "page")} />;
}
