"use client";

import { useCallback, useEffect, useRef } from 'react';

/**
 * Reports a choice once it TOOK EFFECT, not when it was pressed (SPEC.md 27.7: an event is
 * sent only after the action succeeded).
 *
 * `shown` is what the page shows now: the sort a list was loaded with, the visibility the
 * API's sentence names. The returned function records what the visitor just asked for;
 * `report` is called once when `shown` becomes exactly that. A choice whose read failed is
 * never reported, and neither is a change of `shown` nobody asked for.
 */
export function useReportWhenShown<T>(shown: T | undefined, report: (value: T) => void): (asked: T) => void {
  const asked = useRef<{ value: T } | null>(null);
  const latest = useRef(report);
  useEffect(() => {
    latest.current = report;
  });

  useEffect(() => {
    if (asked.current === null || shown === undefined || shown !== asked.current.value) return;
    asked.current = null;
    latest.current(shown);
  }, [shown]);

  return useCallback((value: T) => {
    asked.current = { value };
  }, []);
}
