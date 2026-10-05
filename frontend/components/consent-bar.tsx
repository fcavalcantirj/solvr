"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useConsent } from "@/hooks/use-consent";
import { isUntrackedPath } from "@/lib/analytics-paths";
import { onConsentSettingsRequest, setConsent, type ConsentChoice } from "@/lib/consent";

// The consent bar (SPEC.md 27.7). It asks once, for everyone: Google Analytics loads only
// after Accept.
//
// A region fixed to the bottom of the window, never a dialog: it traps no focus, dims
// nothing and leaves the page readable and clickable around it. It takes no room in the
// layout, so nothing moves when it appears. It shows only while nothing was chosen, only in
// the browser (server HTML is shared between visitors), and never by itself on an untracked
// page. "Cookie settings" brings it back anywhere, with the current choice in words.
//
// Its shape depends on the width. Below the lg breakpoint it is a strip across the bottom
// edge. From lg up a strip that wide covered the home page's primary call to action
// (measured at 1280x800), so there it is a compact card in the bottom right corner: the
// sentence, then the two buttons side by side under it.
//
// The corner depends on the page. /login and /join put their form in the right half from lg
// up, and a card on the right covered SIGN IN and CREATE ACCOUNT (measured from 1024x768 to
// 1440x900). On those two the card takes the left corner, over the brand panel, where it
// covers no control.

// Pages whose main action sits in the bottom right at lg widths: the card goes left there.
const LEFT_CORNER_PATHS = ["/login", "/join"];
const RIGHT_CORNER = "lg:left-auto lg:right-6";
const LEFT_CORNER = "lg:left-6 lg:right-auto";

function cornerFor(pathname: string | null): string {
  const path = pathname && pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
  return path !== null && LEFT_CORNER_PATHS.includes(path) ? LEFT_CORNER : RIGHT_CORNER;
}

// Both choices wear the same control: neither is the one the page would rather have.
const CHOICE =
  "inline-flex min-h-11 w-full items-center justify-center border border-foreground bg-background px-6 py-3 font-mono text-[11px] uppercase leading-none tracking-[0.18em] text-foreground transition-colors hover:bg-foreground hover:text-background focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground";

export function ConsentBar() {
  const consent = useConsent();
  const pathname = usePathname();
  const [reopened, setReopened] = useState(false);
  const region = useRef<HTMLElement>(null);
  const opener = useRef<HTMLElement | null>(null);

  useEffect(
    () =>
      onConsentSettingsRequest(() => {
        opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        setReopened(true);
      }),
    [],
  );

  // Asked for by the visitor: focus goes to the bar so the keyboard and a screen reader are
  // where the answer is. The bar that shows by itself never takes focus.
  useEffect(() => {
    if (reopened) region.current?.focus();
  }, [reopened]);

  const close = useCallback(() => {
    setReopened(false);
    const back = opener.current;
    opener.current = null;
    if (back?.isConnected) back.focus();
  }, []);

  const choose = (choice: ConsentChoice) => {
    setConsent(choice);
    close();
  };

  if (consent === null) return null;
  const asking = consent === "unset" && !isUntrackedPath(pathname);
  if (!asking && !reopened) return null;

  const status = !reopened || consent === "unset" ? null : consent === "granted" ? "Analytics is on." : "Analytics is off.";

  return (
    <section
      ref={region}
      aria-label="Analytics consent"
      tabIndex={-1}
      onKeyDown={(event) => {
        // Only a bar the visitor opened closes on Escape; the question itself stays.
        if (event.key === "Escape" && reopened) close();
      }}
      className={`fixed inset-x-0 bottom-0 z-50 border-t border-border bg-background text-foreground focus:outline-none ${cornerFor(pathname)} lg:bottom-6 lg:w-[26rem] lg:border lg:border-foreground`}
    >
      <div className="mx-auto flex w-full max-w-[84rem] flex-col gap-4 px-4 pb-[max(1rem,env(safe-area-inset-bottom))] pt-4 sm:px-6 md:flex-row md:items-center md:justify-between md:gap-10 lg:flex-col lg:items-stretch lg:justify-start lg:gap-4 lg:px-5 lg:pb-5 lg:pt-5">
        <div className="min-w-0">
          {status ? (
            <p className="mb-1.5 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">{status}</p>
          ) : null}
          <p className="max-w-[68ch] text-sm leading-relaxed">
            Solvr would like to use Google Analytics to learn which pages help. Nothing goes to Google unless you accept.{" "}
            <Link
              href="/privacy"
              className="underline underline-offset-4 transition-colors hover:text-muted-foreground focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground"
            >
              Privacy
            </Link>
          </p>
        </div>
        <div className="grid shrink-0 grid-cols-2 gap-3 md:w-[20rem] lg:w-full">
          <button type="button" onClick={() => choose("denied")} className={CHOICE}>
            Decline
          </button>
          <button type="button" onClick={() => choose("granted")} className={CHOICE}>
            Accept
          </button>
        </div>
      </div>
    </section>
  );
}
