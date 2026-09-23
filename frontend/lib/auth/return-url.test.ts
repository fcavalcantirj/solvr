import { describe, it, expect } from 'vitest';
import { safeReturnPath } from './return-url';

// safeReturnPath backs the "restore the same room after login" behavior (task 39,
// step 3): the room composer sends ?next=/rooms/<slug> to /login, and the login
// flow returns the user there. The guard keeps a crafted ?next= from redirecting
// the user off-site after they authenticate (open-redirect protection).
describe('safeReturnPath', () => {
  it('returns a same-origin relative path unchanged', () => {
    expect(safeReturnPath('/rooms/tictactoe-human-vs-computer-20260920')).toBe(
      '/rooms/tictactoe-human-vs-computer-20260920',
    );
  });

  it('preserves a query string on the relative path', () => {
    expect(safeReturnPath('/rooms/demo?message=42')).toBe('/rooms/demo?message=42');
  });

  it('rejects null and empty', () => {
    expect(safeReturnPath(null)).toBeNull();
    expect(safeReturnPath(undefined)).toBeNull();
    expect(safeReturnPath('')).toBeNull();
  });

  it('rejects an absolute URL (open-redirect)', () => {
    expect(safeReturnPath('https://evil.example/steal')).toBeNull();
    expect(safeReturnPath('http://evil.example')).toBeNull();
  });

  it('rejects a protocol-relative //host', () => {
    expect(safeReturnPath('//evil.example')).toBeNull();
  });

  it('rejects a backslash-smuggled /\\host', () => {
    expect(safeReturnPath('/\\evil.example')).toBeNull();
  });

  it('rejects a path without a leading slash', () => {
    expect(safeReturnPath('rooms/x')).toBeNull();
  });
});
