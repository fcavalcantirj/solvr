// The visitor's choice about Google Analytics (SPEC.md 27.7).
//
// It lives in this browser and nowhere else. Pages are served from shared caches, so the
// server never reads it and never renders anything that depends on it: this module is
// called from the browser only, and answers "unset" anywhere else.
//
// Three states. "unset": nothing was chosen, the consent bar asks. "granted" and "denied":
// the visitor chose, and the choice does not expire. A browser that sends Global Privacy
// Control and has no stored choice counts as denied without being asked; a stored choice
// always wins over the signal.

export const CONSENT_STORAGE_KEY = 'solvr_consent';

export type ConsentChoice = 'granted' | 'denied';
export type ConsentState = 'unset' | ConsentChoice;

interface ConsentRecord {
  analytics: ConsentChoice;
  at: string;
  v: 1;
}

// A choice the storage refused to keep (private mode, blocked site data, a full quota). It
// holds for as long as this page lives and wins over whatever older record is still stored;
// the next page starts unset again, because nothing was kept.
let unsavedChoice: ConsentRecord | null = null;

const consentListeners = new Set<() => void>();
const settingsListeners = new Set<() => void>();

function isRecord(value: unknown): value is ConsentRecord {
  if (!value || typeof value !== 'object') return false;
  const record = value as Record<string, unknown>;
  return (
    record.v === 1 &&
    (record.analytics === 'granted' || record.analytics === 'denied') &&
    typeof record.at === 'string'
  );
}

function readStored(): ConsentRecord | null {
  try {
    const raw = window.localStorage.getItem(CONSENT_STORAGE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    return isRecord(parsed) ? parsed : null;
  } catch {
    // Storage that throws, or text that is not a record: nothing was chosen.
    return null;
  }
}

function sendsGlobalPrivacyControl(): boolean {
  return (window.navigator as Navigator & { globalPrivacyControl?: unknown }).globalPrivacyControl === true;
}

/** The state right now. Read on every call, so a choice made in another tab holds here too. */
export function getConsent(): ConsentState {
  if (typeof window === 'undefined') return 'unset';
  const record = unsavedChoice ?? readStored();
  if (record) return record.analytics;
  return sendsGlobalPrivacyControl() ? 'denied' : 'unset';
}

/** Record the visitor's choice and tell the page. Never throws. */
export function setConsent(choice: ConsentChoice): void {
  if (typeof window === 'undefined') return;
  const record: ConsentRecord = { analytics: choice, at: new Date().toISOString(), v: 1 };
  try {
    window.localStorage.setItem(CONSENT_STORAGE_KEY, JSON.stringify(record));
    unsavedChoice = null;
  } catch {
    unsavedChoice = record;
  }
  notify(consentListeners);
}

function notify(listeners: Set<() => void>): void {
  for (const listener of [...listeners]) listener();
}

function onStorage(event: StorageEvent): void {
  // key is null when the whole storage was cleared.
  if (event.key === CONSENT_STORAGE_KEY || event.key === null) notify(consentListeners);
}

/** Be told when the state may have changed: here, or in another tab. Returns the unsubscribe. */
export function subscribeConsent(listener: () => void): () => void {
  if (typeof window === 'undefined') return () => {};
  if (consentListeners.size === 0) window.addEventListener('storage', onStorage);
  consentListeners.add(listener);
  return () => {
    consentListeners.delete(listener);
    if (consentListeners.size === 0) window.removeEventListener('storage', onStorage);
  };
}

/** "Cookie settings": ask the consent bar to show itself again, whatever the state. */
export function openConsentSettings(): void {
  notify(settingsListeners);
}

/** Be told when "Cookie settings" was pressed. Returns the unsubscribe. */
export function onConsentSettingsRequest(listener: () => void): () => void {
  settingsListeners.add(listener);
  return () => {
    settingsListeners.delete(listener);
  };
}
