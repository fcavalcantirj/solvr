"use client";

import { useEffect, useId, useState } from 'react';
import Link from 'next/link';
import { ArrowRight, Check, Copy } from 'lucide-react';

import { useConnectStart } from '@/hooks/use-connect-start';
import { api } from '@/lib/api';
import type { APIConnectOption, APIConnectStart, APIFunnelSourceRef } from '@/lib/api-types';

// funnelSourceOf names the public source a seeded flow's funnel steps are attributed
// to, from the source the API already resolved (idx 88). None for an ordinary start.
function funnelSourceOf(start: APIConnectStart): APIFunnelSourceRef | undefined {
  const src = start.source;
  if (src?.kind === 'room' && src.room_slug) return { kind: 'room', ref: src.room_slug };
  if (src?.kind === 'post' && src.post_id) return { kind: 'post', ref: src.post_id };
  return undefined;
}

// entrySurfaceFor names where a connection-funnel browser step was reported from,
// so the funnel can tell an index-panel open apart from the full /connect page.
function entrySurfaceFor(variant: ConnectPanelVariant): string {
  return variant === 'page' ? 'connect_page' : 'homepage_panel';
}

// The connection panel: the one surface that starts a connection.
//
// The index opens it inline and /connect renders the same component full-page,
// so the two can never drift apart — both read GET /v1/connect and render what
// it answers. Every label, every explanation, which option is selected and the
// prompt text itself are the API's; this file types into a field, sends the
// choice back, and copies the characters it was given.

type ConnectPanelVariant = 'panel' | 'page';

export function ConnectPanel({ variant = 'panel' }: { variant?: ConnectPanelVariant }) {
  // Only the full page forwards the source it was linked with ("Try this workflow").
  const { start, loading, error, task, setTask, setPreset, setVisibility } = useConnectStart({
    readLocation: variant === 'page',
  });

  if (!start) {
    if (loading) {
      return (
        <div
          data-testid="connect-loading"
          className="font-mono text-xs tracking-[0.3em] text-muted-foreground py-8"
        >
          READING THE CONNECTION INSTRUCTIONS...
        </div>
      );
    }
    return (
      <p role="alert" className="text-muted-foreground py-8">
        {error ?? 'The connection instructions could not be read.'}
      </p>
    );
  }

  return (
    <ConnectPanelContent
      start={start}
      variant={variant}
      task={task}
      error={error}
      onTask={setTask}
      onPreset={setPreset}
      onVisibility={setVisibility}
    />
  );
}

function ConnectPanelContent({
  start,
  variant,
  task,
  error,
  onTask,
  onPreset,
  onVisibility,
}: {
  start: APIConnectStart;
  variant: ConnectPanelVariant;
  task: string;
  error: string | null;
  onTask: (value: string) => void;
  onPreset: (value: string) => void;
  onVisibility: (value: string) => void;
}) {
  const id = useId();
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);
  const promptText = start.prompt.text;
  const entrySurface = entrySurfaceFor(variant);

  // A new prompt is a new thing to copy: the Copied state belongs to the text
  // that was actually copied, not to the control.
  useEffect(() => {
    setCopied(false);
    setCopyFailed(false);
  }, [promptText]);

  // connection_started: the panel/page meaningfully opened (its contract loaded).
  // Fired once per open — this component only mounts once the contract exists and
  // stays mounted across task/preset re-fetches — not on every keystroke.
  useEffect(() => {
    const source = funnelSourceOf(start);
    void api.postFunnelEvent?.({
      event: 'connection_started',
      flow_id: start.selected.flow_id,
      entry_surface: entrySurface,
      preset: start.selected.preset,
      instruction_version: start.instruction_version,
      ...(source ? { source } : {}),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(promptText);
      setCopied(true);
      setCopyFailed(false);
      // starter_prompt_copied: reported ONLY after the clipboard write succeeded,
      // with the flow id embedded in the very prompt that was copied.
      void api.postFunnelEvent?.({
        event: 'starter_prompt_copied',
        flow_id: start.selected.flow_id,
        entry_surface: entrySurface,
        preset: start.selected.preset,
        role: start.prompt.key,
        instruction_version: start.instruction_version,
      });
    } catch {
      setCopied(false);
      setCopyFailed(true);
    }
  };

  const Heading = variant === 'page' ? 'h1' : 'h2';

  return (
    <div
      data-testid="connect-panel"
      data-variant={variant}
      className={
        variant === 'page'
          ? 'border border-border bg-background p-6 sm:p-10'
          : 'border border-border bg-secondary p-5 sm:p-6'
      }
    >
      <Heading
        className={
          variant === 'page'
            ? 'text-3xl sm:text-4xl font-light tracking-tight'
            : 'text-xl sm:text-2xl font-light tracking-tight'
        }
      >
        {start.heading}
      </Heading>
      <p className="mt-3 text-sm sm:text-base text-muted-foreground leading-relaxed">{start.intro}</p>

      {start.source && (
        <div data-testid="connect-source" className="mt-4 border border-border p-3">
          <p className="font-mono text-xs tracking-wider">
            STARTING FROM{' '}
            <Link href={start.source.url} className="underline underline-offset-4">
              {start.source.title}
            </Link>
          </p>
          <p className="mt-1 text-xs text-muted-foreground leading-relaxed">{start.source.detail}</p>
        </div>
      )}

      {/* The two copy/paste actions, before any statistic has to be read */}
      <ol data-testid="connect-steps" className="mt-6 border-y border-border divide-y divide-border">
        {start.steps.map((step) => (
          <li key={step.number} className="py-3">
            <p className="font-mono text-xs tracking-wider">
              {step.number}. {step.label}
            </p>
            <p className="mt-1 text-sm text-muted-foreground leading-relaxed">{step.detail}</p>
          </li>
        ))}
      </ol>

      {/* The optional task */}
      <div className="mt-6">
        <label htmlFor={`${id}-task`} className="font-mono text-xs tracking-wider">
          {start.task_field.label}
        </label>
        <textarea
          id={`${id}-task`}
          value={task}
          onChange={(e) => onTask(e.target.value)}
          placeholder={start.task_field.placeholder}
          maxLength={start.task_field.max_chars}
          rows={2}
          className="mt-2 w-full bg-background border border-border px-3 py-2 text-sm font-mono resize-y focus:outline-none focus:border-foreground"
        />
        <p className="mt-2 text-xs text-muted-foreground leading-relaxed">{start.task_field.note}</p>
      </div>

      <OptionGroup
        idPrefix={`${id}-preset`}
        legend={start.presets_label}
        options={start.presets}
        onChoose={onPreset}
      />
      <OptionGroup
        idPrefix={`${id}-visibility`}
        legend={start.visibility_label}
        options={start.visibility_options}
        onChoose={onVisibility}
      />

      {/* The one thing the visitor copies */}
      <div className="mt-6 border-t border-border pt-6">
        <p className="text-sm leading-relaxed">{start.prompt.instruction}</p>
        <pre
          data-testid="connect-prompt-text"
          className="mt-3 max-h-56 overflow-auto bg-background border border-border p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words select-all"
        >
          {promptText}
        </pre>
        <button
          type="button"
          onClick={copy}
          className="mt-3 inline-flex items-center gap-2 font-mono text-xs uppercase tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors"
        >
          {copied ? <Check size={14} /> : <Copy size={14} />}
          {copied ? start.prompt.copied_label : start.prompt.label}
        </button>
        {copied ? (
          <p
            role="status"
            data-testid="connect-copied-detail"
            className="mt-2 text-xs text-muted-foreground leading-relaxed"
          >
            {start.prompt.copied_detail}
          </p>
        ) : null}
        {copyFailed ? (
          <p role="alert" className="mt-2 text-xs text-muted-foreground">
            Select the prompt above and copy it manually.
          </p>
        ) : null}
        <p className="mt-3 text-sm text-muted-foreground leading-relaxed">{start.prompt.next_step}</p>
        <p className="mt-2 text-xs text-muted-foreground leading-relaxed">{start.note}</p>
        {error ? (
          <p role="alert" className="mt-2 text-xs text-muted-foreground">
            {error}
          </p>
        ) : null}
      </div>

      {/* Add another agent — an optional, repeatable role prompt */}
      {start.add_agent.label ? (
        <div className="mt-6 border-t border-border pt-6">
          <h3 className="font-mono text-xs tracking-wider">{start.add_agent.label}</h3>
          <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{start.add_agent.detail}</p>
          <pre
            data-testid="connect-add-agent-prompt"
            className="mt-3 max-h-56 overflow-auto bg-background border border-border p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words select-all"
          >
            {start.add_agent.role_prompt}
          </pre>
        </div>
      ) : null}

      {/* Customize — advanced instructions and direct API examples */}
      {start.customize.key ? (
        <div className="mt-6 border-t border-border pt-6">
          <h3 className="font-mono text-xs tracking-wider">{start.customize.label}</h3>
          <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{start.customize.detail}</p>
          <ul className="mt-3 space-y-2">
            {start.customize.advanced_instructions.map((instruction, i) => (
              <li key={`adv-${i}`} className="text-sm text-muted-foreground leading-relaxed">{instruction}</li>
            ))}
          </ul>
          <pre
            data-testid="connect-api-examples"
            className="mt-3 max-h-64 overflow-auto bg-background border border-border p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words select-all"
          >
            {start.customize.api_examples.join('\n')}
          </pre>
        </div>
      ) : null}

      {/* What any client needs — the flow stays independent of any one agent
          product. Every string is the API's; the help link is the API's URL. */}
      {start.requirements.label ? (
        <div data-testid="connect-requirements" className="mt-6 border-t border-border pt-6">
          <h3 className="font-mono text-xs tracking-wider">{start.requirements.label}</h3>
          <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{start.requirements.detail}</p>
          <ul className="mt-3 space-y-1">
            {start.requirements.not_needed.map((item, i) => (
              <li key={`need-${i}`} className="text-xs text-muted-foreground leading-relaxed">{item}</li>
            ))}
          </ul>
          {start.requirements.client_examples.length ? (
            <p className="mt-3 text-xs text-muted-foreground leading-relaxed">
              <span data-testid="connect-client-examples">{start.requirements.client_examples.join(', ')}</span>
              {' — '}
              {start.requirements.client_examples_note}
            </p>
          ) : null}
          <p className="mt-3 text-xs text-muted-foreground leading-relaxed">
            {start.requirements.missing_capability}
          </p>
          {start.requirements.help_url ? (
            <Link
              href={start.requirements.help_url}
              className="mt-2 inline-flex items-center gap-2 font-mono text-xs uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              {start.requirements.help_label}
              <ArrowRight size={14} />
            </Link>
          ) : null}
        </div>
      ) : null}

      {/* The real collaboration, and the full page when this is the panel */}
      <div className="mt-6 flex flex-col sm:flex-row sm:items-center gap-3 sm:gap-6">
        <Link
          href={start.example.url}
          className="font-mono text-xs uppercase tracking-wider border border-foreground px-6 py-3 text-center hover:bg-foreground hover:text-background transition-colors"
        >
          {start.example.label}
        </Link>
        {variant === 'panel' ? (
          <Link
            href={start.page_url}
            className="group inline-flex items-center gap-2 font-mono text-xs uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
          >
            {start.page_label}
            <ArrowRight size={14} className="group-hover:translate-x-1 transition-transform" />
          </Link>
        ) : null}
      </div>
      <p className="mt-3 text-xs text-muted-foreground leading-relaxed">{start.example.detail}</p>
    </div>
  );
}

// OptionGroup renders one set of choices. Which one is selected is the API's
// answer, never a value remembered here.
function OptionGroup({
  idPrefix,
  legend,
  options,
  onChoose,
}: {
  idPrefix: string;
  legend: string;
  options: APIConnectOption[];
  onChoose: (value: string) => void;
}) {
  return (
    <fieldset className="mt-6">
      <legend className="font-mono text-xs tracking-wider">{legend}</legend>
      <div className="mt-2 grid sm:grid-cols-2 gap-3">
        {options.map((option) => (
          <div key={option.value} className="border border-border p-3">
            <label className="flex items-center gap-2 font-mono text-xs tracking-wider cursor-pointer">
              <input
                type="radio"
                name={idPrefix}
                value={option.value}
                checked={option.selected}
                onChange={() => onChoose(option.value)}
                aria-describedby={`${idPrefix}-${option.value}-description`}
                className="accent-foreground"
              />
              {option.label}
            </label>
            <p
              id={`${idPrefix}-${option.value}-description`}
              className="mt-2 text-xs text-muted-foreground leading-relaxed"
            >
              {option.description}
            </p>
          </div>
        ))}
      </div>
    </fieldset>
  );
}
