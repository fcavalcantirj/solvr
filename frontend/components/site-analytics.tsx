"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { useConsent } from "@/hooks/use-consent";
import { dropTrackedEvents, flushTrackedEvents } from "@/lib/analytics";
import { contentGroupForPath, isUntrackedPath } from "@/lib/analytics-paths";
import {
  deleteGoogleAnalyticsCookies,
  loadGoogleTag,
  setGoogleTagConsent,
  setGoogleTagContentGroup,
  setGoogleTagEnabled,
} from "@/lib/google-tag";
import { whenPageIdle } from "@/lib/when-page-idle";

// The untracked-path rule lives in lib/analytics-paths.ts; it is named here too because
// this is the component that enforces it.
export { isUntrackedPath };

/**
 * SiteAnalytics decides WHEN Google's tag may be in the page (SPEC.md 27.7). It is requested
 * only when three things hold at once: the visitor accepted analytics, the page is not an
 * untracked one, and the page is idle. No consent: no tag, no request to Google, no cookie.
 *
 * The choice is known in the browser only, so this renders nothing, on the server or after.
 * A tag cannot be taken out of a page again; when it must stop (Decline after Accept, a move
 * to an untracked page) Google's own off switch silences it.
 */
export function SiteAnalytics({ gaId }: { gaId: string }) {
  const pathname = usePathname();
  const consent = useConsent();
  const known = consent !== null;
  const allowed = consent === "granted" && !isUntrackedPath(pathname);

  // The switch, set from every known state. Off comes first: nothing after it is sent.
  // Without consent nothing waits to be sent and no Google Analytics cookie stays, whether
  // it was set a moment ago (Decline after Accept) or on a visit before the bar existed.
  useEffect(() => {
    if (!known) return;
    setGoogleTagEnabled(gaId, allowed);
    setGoogleTagConsent(consent === "granted");
    if (consent !== "granted") {
      dropTrackedEvents();
      deleteGoogleAnalyticsCookies();
    }
  }, [gaId, known, allowed, consent]);

  // The tag itself: last thing the page asks for, and only while it is allowed.
  useEffect(() => {
    if (!allowed) return;
    return whenPageIdle(() => {
      loadGoogleTag(gaId, contentGroupForPath(window.location.pathname));
      flushTrackedEvents();
    });
  }, [gaId, allowed]);

  // A client-side navigation: the tag sends its own page view about a second after the
  // address changes, and carries the kind of page it was told by then.
  useEffect(() => {
    if (!allowed) return;
    setGoogleTagContentGroup(contentGroupForPath(pathname));
    flushTrackedEvents();
  }, [allowed, pathname]);

  return null;
}
