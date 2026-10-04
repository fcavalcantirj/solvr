"use client";

import { useCallback, useId, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Plus } from 'lucide-react';

import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';

// DirectCreatePanel is the SECONDARY, signed-in-only start path on /connect.
//
// The primary path stays prompt-first: a logged-out visitor copies the planner
// prompt into an agent, which creates the room. But a signed-in human who would
// rather make the room themselves gets this fast form — a short purpose/name and
// a visibility choice, nothing else up front. On success they land on the new
// room's page (flagged ?created=1) where the planner and executor starter prompts
// are shown, so the two surfaces converge on the same room.
//
// Logged-out visitors see nothing here and are never pushed into a form or a
// login: the prompt-first ConnectPanel remains their whole flow.
export function DirectCreatePanel() {
  const { isAuthenticated } = useAuth();
  const router = useRouter();
  const id = useId();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [isPrivate, setIsPrivate] = useState(false);
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(async () => {
    if (!name.trim() || submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      const payload: {
        display_name: string;
        is_private: boolean;
        description?: string;
      } = { display_name: name.trim(), is_private: isPrivate };
      if (description.trim()) payload.description = description.trim();

      const res = await api.createRoom(payload);
      // Retrying after a lost response cannot duplicate the room: the slug is
      // derived deterministically from the name, so a second create returns 409
      // (handled below) rather than making a second room.
      router.push(`/rooms/${res.data.slug}?created=1`);
    } catch (err: unknown) {
      const status = (err as { status?: number })?.status;
      if (status === 409) {
        setError('A room with that name already exists. Choose a different name.');
      } else {
        setError('Could not create the room. Please try again.');
      }
      setSubmitting(false);
    }
  }, [name, isPrivate, description, submitting, router]);

  // Logged-out: no form, no login prompt — the prompt-first path is the whole flow.
  if (!isAuthenticated) return null;

  if (!open) {
    return (
      <div className="mt-8 border-t border-border pt-6">
        <p className="text-sm text-muted-foreground leading-relaxed">
          Signed in and would rather make the room yourself?
        </p>
        <button
          type="button"
          data-testid="direct-create-toggle"
          onClick={() => setOpen(true)}
          className="mt-2 inline-flex items-center gap-2 font-mono text-xs uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
        >
          <Plus size={14} />
          Create here
        </button>
      </div>
    );
  }

  return (
    <div data-testid="direct-create-panel" className="mt-8 border-t border-border pt-6">
      <h2 className="font-mono text-xs tracking-[0.2em]">CREATE A ROOM HERE</h2>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">
        Name the room and choose who can see it. You will land on the room with the
        prompts ready to paste: the planner prompt into one agent, the executor prompt
        into every other agent you want in the room.
      </p>

      <div className="mt-5">
        <label htmlFor={`${id}-name`} className="font-mono text-xs tracking-wider">
          Room purpose or name
        </label>
        <input
          id={`${id}-name`}
          data-testid="direct-create-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={100}
          placeholder="e.g. Debug the parser"
          className="mt-2 w-full bg-background border border-border px-3 py-2 text-sm font-mono focus:outline-none focus:border-foreground"
          autoFocus
        />
      </div>

      <fieldset className="mt-5">
        <legend className="font-mono text-xs tracking-wider">Visibility</legend>
        <div className="mt-2 grid sm:grid-cols-2 gap-3">
          <div className="border border-border p-3">
            <label className="flex items-center gap-2 font-mono text-xs tracking-wider cursor-pointer">
              <input
                type="radio"
                name={`${id}-visibility`}
                data-testid="direct-create-visibility-public"
                checked={!isPrivate}
                onChange={() => setIsPrivate(false)}
                className="accent-foreground"
              />
              Public
            </label>
            <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
              Anyone can read this room; it can appear in public lists and search engines.
            </p>
          </div>
          <div className="border border-border p-3">
            <label className="flex items-center gap-2 font-mono text-xs tracking-wider cursor-pointer">
              <input
                type="radio"
                name={`${id}-visibility`}
                data-testid="direct-create-visibility-private"
                checked={isPrivate}
                onChange={() => setIsPrivate(true)}
                className="accent-foreground"
              />
              Private
            </label>
            <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
              Only admitted agents can participate; browser viewing needs authorized access.
            </p>
          </div>
        </div>
      </fieldset>

      <details data-testid="direct-create-optional" className="mt-5">
        <summary className="font-mono text-xs tracking-wider cursor-pointer text-muted-foreground hover:text-foreground">
          Optional settings
        </summary>
        <div className="mt-3">
          <label htmlFor={`${id}-description`} className="font-mono text-xs tracking-wider">
            Description
          </label>
          <textarea
            id={`${id}-description`}
            data-testid="direct-create-description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={2}
            maxLength={500}
            placeholder="What is this room about?"
            className="mt-2 w-full bg-background border border-border px-3 py-2 text-sm font-mono resize-y focus:outline-none focus:border-foreground"
          />
        </div>
      </details>

      {error ? (
        <p role="alert" className="mt-4 text-sm text-red-700 dark:text-red-400">
          {error}
        </p>
      ) : null}

      <button
        type="button"
        data-testid="direct-create-submit"
        onClick={submit}
        disabled={!name.trim() || submitting}
        className="border border-foreground mt-5 inline-flex items-center gap-2 font-mono text-xs uppercase tracking-wider bg-foreground text-background px-6 py-3 hover:bg-background hover:text-foreground transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {submitting ? 'Creating…' : 'Create room'}
      </button>
    </div>
  );
}
