import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { IDLE_FALLBACK_MS, IDLE_TIMEOUT_MS, whenPageIdle } from './when-page-idle';

// Google's tag is the last thing a page asks for: after the window's load event, and then
// only when the browser says it has nothing better to do (SPEC.md 27.7).

type IdleWindow = {
  requestIdleCallback?: (callback: () => void, options?: { timeout: number }) => number;
  cancelIdleCallback?: (handle: number) => void;
};

const w = () => window as unknown as IdleWindow;

function setReadyState(state: DocumentReadyState) {
  Object.defineProperty(document, 'readyState', { value: state, configurable: true });
}

describe('whenPageIdle', () => {
  let idleCallbacks: Array<() => void>;
  let idleOptions: Array<{ timeout: number } | undefined>;

  beforeEach(() => {
    idleCallbacks = [];
    idleOptions = [];
    w().requestIdleCallback = vi.fn((callback: () => void, options?: { timeout: number }) => {
      idleCallbacks.push(callback);
      idleOptions.push(options);
      return idleCallbacks.length;
    });
    w().cancelIdleCallback = vi.fn();
    setReadyState('complete');
  });

  afterEach(() => {
    delete w().requestIdleCallback;
    delete w().cancelIdleCallback;
    setReadyState('complete');
    vi.useRealTimers();
  });

  it('waits for the browser to be idle, for at most four seconds, on a page that has loaded', () => {
    const callback = vi.fn();
    whenPageIdle(callback);

    expect(callback).not.toHaveBeenCalled();
    expect(w().requestIdleCallback).toHaveBeenCalledTimes(1);
    expect(idleOptions[0]).toEqual({ timeout: 4000 });
    expect(IDLE_TIMEOUT_MS).toBe(4000);

    idleCallbacks[0]();
    expect(callback).toHaveBeenCalledTimes(1);
  });

  it('waits for the load event first on a page that is still loading', () => {
    setReadyState('interactive');
    const callback = vi.fn();
    whenPageIdle(callback);

    expect(w().requestIdleCallback).not.toHaveBeenCalled();

    window.dispatchEvent(new Event('load'));
    expect(w().requestIdleCallback).toHaveBeenCalledTimes(1);
    expect(callback).not.toHaveBeenCalled();

    idleCallbacks[0]();
    expect(callback).toHaveBeenCalledTimes(1);

    // A second load event (there is none in a browser) starts nothing new.
    window.dispatchEvent(new Event('load'));
    expect(w().requestIdleCallback).toHaveBeenCalledTimes(1);
  });

  it('falls back to a timer where the browser has no requestIdleCallback', () => {
    delete w().requestIdleCallback;
    delete w().cancelIdleCallback;
    vi.useFakeTimers();
    setReadyState('loading');
    const callback = vi.fn();
    whenPageIdle(callback);

    vi.advanceTimersByTime(60_000);
    expect(callback).not.toHaveBeenCalled(); // still loading

    window.dispatchEvent(new Event('load'));
    vi.advanceTimersByTime(IDLE_FALLBACK_MS - 1);
    expect(callback).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(callback).toHaveBeenCalledTimes(1);
  });

  it('can be cancelled before the page has loaded', () => {
    setReadyState('loading');
    const callback = vi.fn();
    const cancel = whenPageIdle(callback);

    cancel();
    window.dispatchEvent(new Event('load'));
    expect(w().requestIdleCallback).not.toHaveBeenCalled();
    expect(callback).not.toHaveBeenCalled();
  });

  it('can be cancelled while it waits for the idle moment', () => {
    const callback = vi.fn();
    const cancel = whenPageIdle(callback);

    cancel();
    expect(w().cancelIdleCallback).toHaveBeenCalledWith(1);
    // Even a browser that calls back anyway reaches nothing.
    idleCallbacks[0]();
    expect(callback).not.toHaveBeenCalled();
  });

  it('can be cancelled while the fallback timer runs', () => {
    delete w().requestIdleCallback;
    delete w().cancelIdleCallback;
    vi.useFakeTimers();
    const callback = vi.fn();
    const cancel = whenPageIdle(callback);

    cancel();
    vi.advanceTimersByTime(60_000);
    expect(callback).not.toHaveBeenCalled();
  });
});
