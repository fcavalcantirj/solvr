"use client";

import { useSyncExternalStore } from "react";
import { getConsent, subscribeConsent, type ConsentState } from "@/lib/consent";

// On the server, and in the browser until the page has hydrated, the choice is not known.
const notKnownYet = () => null;

/**
 * The visitor's choice about analytics, kept current: "unset", "granted" or "denied", and
 * null while it is not known yet. Server HTML is shared between visitors, so nothing that
 * depends on the choice may render before hydration; a component that gets null renders
 * what it would for nobody.
 */
export function useConsent(): ConsentState | null {
  return useSyncExternalStore<ConsentState | null>(subscribeConsent, getConsent, notKnownYet);
}
