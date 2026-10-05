"use client";

import { useEffect } from "react";
import { track } from "@/lib/analytics";

const EVENT_FOR_MARK = { nav: "nav_click", cta: "cta_click" } as const;

/**
 * The site's one click listener (SPEC.md 27.7), mounted once in the root layout. A click on,
 * or inside, an element marked with trackNav() or trackCta() (lib/track-attrs.ts) sends
 * nav_click or cta_click with the element's item and location. track() decides whether
 * anything leaves the browser: without consent, nothing does.
 *
 * It listens in the capture phase, so a control that stops its own click from spreading is
 * still counted.
 */
export function TrackClicks() {
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      const marked = event.target instanceof Element ? event.target.closest("[data-track]") : null;
      if (!marked) return;
      const mark = marked.getAttribute("data-track");
      if (mark !== "nav" && mark !== "cta") return;
      track(EVENT_FOR_MARK[mark], {
        item: marked.getAttribute("data-track-item") ?? undefined,
        location: marked.getAttribute("data-track-location") ?? undefined,
      });
    };
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, []);

  return null;
}
