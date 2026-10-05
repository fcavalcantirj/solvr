"use client";

import { useEffect } from 'react';
import { reportShareVisit } from '@/lib/funnel';
import type { APIFunnelSourceRef } from '@/lib/api-types';

// share_visit (idx 88): a page opened through a share link (?via=share) is counted
// ONCE per browser tab, attributed to the public room or post it shows — a visit,
// never a person: no identifier is created or stored beyond this tab's session.
// The marker is then removed from the address bar so the page shows its clean link.
// A page that cannot be attributed (a private room: source null) only cleans the URL.

const VIA_PARAM = 'via';
const VIA_SHARE = 'share';

function stripVia() {
  const url = new URL(window.location.href);
  url.searchParams.delete(VIA_PARAM);
  window.history.replaceState(window.history.state, '', url.pathname + url.search + url.hash);
}

export function useShareVisit(source: APIFunnelSourceRef | null, entrySurface: string) {
  const kind = source?.kind;
  const ref = source?.ref;

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (new URLSearchParams(window.location.search).get(VIA_PARAM) !== VIA_SHARE) return;

    if (kind && ref) {
      const key = `solvr_share_visit:${kind}:${ref}`;
      let seen = false;
      try {
        seen = window.sessionStorage.getItem(key) === '1';
        if (!seen) window.sessionStorage.setItem(key, '1');
      } catch {
        // Storage unavailable: count this visit; a reload may count again.
      }
      if (!seen) reportShareVisit({ source: { kind, ref }, surface: entrySurface });
    }
    stripVia();
  }, [kind, ref, entrySurface]);
}
