"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { track } from "@/lib/analytics";

/**
 * Says that a page which does not exist was shown (SPEC.md 27.7: page_not_found). The
 * not-found page is rendered on the server and cannot report anything itself, so this is the
 * small client piece that does. It renders nothing and sends nothing of its own: the
 * address that was asked for is already in the page view. One miss is one event; another
 * missing address reached without a page load is another.
 */
export function PageNotFoundSignal() {
  const pathname = usePathname();

  useEffect(() => {
    track("page_not_found");
  }, [pathname]);

  return null;
}
