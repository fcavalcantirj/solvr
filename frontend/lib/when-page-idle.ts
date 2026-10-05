// "When the page is idle" (SPEC.md 27.7): after the window's load event, then at the
// browser's next idle moment, waiting for one at most four seconds. Browser only.

export const IDLE_TIMEOUT_MS = 4000;
// Where requestIdleCallback is missing (Safari), a short pause after the load event stands
// in for the idle moment.
export const IDLE_FALLBACK_MS = 200;

// Not every browser has these two, whatever the DOM typings say.
type IdleWindow = {
  requestIdleCallback?: (callback: () => void, options?: { timeout: number }) => number;
  cancelIdleCallback?: (handle: number) => void;
};

/** Call back once when the page has loaded and is idle. Returns the cancel. */
export function whenPageIdle(callback: () => void): () => void {
  const w = window as unknown as IdleWindow;
  let cancelled = false;
  let idleHandle: number | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const run = () => {
    if (!cancelled) callback();
  };

  const waitForIdle = () => {
    if (cancelled) return;
    if (typeof w.requestIdleCallback === 'function') {
      idleHandle = w.requestIdleCallback(run, { timeout: IDLE_TIMEOUT_MS });
    } else {
      timer = setTimeout(run, IDLE_FALLBACK_MS);
    }
  };

  if (document.readyState === 'complete') {
    waitForIdle();
  } else {
    window.addEventListener('load', waitForIdle, { once: true });
  }

  return () => {
    cancelled = true;
    window.removeEventListener('load', waitForIdle);
    if (idleHandle !== undefined) w.cancelIdleCallback?.(idleHandle);
    if (timer !== undefined) clearTimeout(timer);
  };
}
