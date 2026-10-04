"use client";

import { cn } from "@/lib/utils";
import { roles } from "./prompt-align";
import type { PromptPreset } from "./prompt-types";

interface RolePairSwitchProps {
  presets: PromptPreset[];
  value: string;
  onChange: (value: string) => void;
  // A closing cell that is not a choice: the pairs are examples, not the limit.
  more?: { label: string; detail: string };
}

// RolePairSwitch picks the use case. It is a radio group: one pair is always on.
export function RolePairSwitch({ presets, value, onChange, more }: RolePairSwitchProps) {
  return (
    <fieldset>
      <legend className="sr-only">Use case</legend>
      <div
        className={cn(
          "grid gap-px border border-border bg-border",
          more ? "grid-cols-2 sm:grid-cols-4" : "grid-cols-3",
        )}
      >
        {presets.map((preset) => {
          const active = preset.value === value;
          const pair = roles(preset.prompt);
          return (
            <label
              key={preset.value}
              className={cn(
                "relative flex cursor-pointer flex-col justify-center gap-1 bg-card px-3 py-3 sm:px-5 sm:py-4",
                "transition-colors has-[input:focus-visible]:outline-2 has-[input:focus-visible]:-outline-offset-2 has-[input:focus-visible]:outline-foreground",
                active
                  ? "text-foreground shadow-[inset_0_-3px_0_var(--foreground)]"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground",
              )}
            >
              <input
                type="radio"
                name="use-case"
                value={preset.value}
                checked={active}
                onChange={() => onChange(preset.value)}
                className="sr-only"
              />
              <span className="text-sm leading-tight sm:text-base">{preset.label}</span>
              {pair.a && pair.b ? (
                <span
                  className={cn(
                    "hidden font-mono text-[11px] uppercase tracking-[0.14em] sm:block",
                    active ? "text-foreground" : "text-muted-foreground",
                  )}
                >
                  {pair.a}
                  <span aria-hidden="true"> · </span>
                  {pair.b}
                </span>
              ) : null}
            </label>
          );
        })}
        {more ? (
          <div
            data-testid="role-pair-more"
            className="flex cursor-default flex-col justify-center gap-1 bg-background px-3 py-3 sm:px-5 sm:py-4"
          >
            <span className="text-sm leading-tight text-foreground sm:text-base">{more.label}</span>
            <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
              {more.detail}
            </span>
          </div>
        ) : null}
      </div>
    </fieldset>
  );
}
