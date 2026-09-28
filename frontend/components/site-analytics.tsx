"use client";

import { usePathname } from "next/navigation";
import { GoogleAnalytics } from "@next/third-parties/google";

// GA sends the page address, query string included, with every hit, so these pages never load
// the tag: their address carries a secret the page is about to use.
//   /claim             #token=<agent claim token>, usable for hours
//   /auth/callback     ?code=<one-time login code>
//   /email/unsubscribe ?email=&token=<unsubscribe token>
// gtag.js dropped the #fragment when measured (2026-09-28), but that is Google's code to
// change, so /claim is listed too. Moving on to any other page loads the tag with that page's
// own address.
const UNTRACKED_PATHS = ["/claim", "/auth/callback", "/email/unsubscribe"];

export function isUntrackedPath(pathname: string | null): boolean {
  if (pathname === null) return true;
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
  return UNTRACKED_PATHS.includes(path);
}

export function SiteAnalytics({ gaId }: { gaId: string }) {
  const pathname = usePathname();
  if (isUntrackedPath(pathname)) return null;
  return <GoogleAnalytics gaId={gaId} />;
}
