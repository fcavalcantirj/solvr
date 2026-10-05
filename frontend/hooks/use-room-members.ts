import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import type { APIRoomMember } from '@/lib/api-types';

interface UseRoomMembersOptions {
  // false: ask for nothing. The member list is an owner-only read, so a page that
  // anyone can open enables it only for a signed-in viewer.
  enabled?: boolean;
}

interface UseRoomMembersResult {
  members: APIRoomMember[];
  loading: boolean;
  error: Error | null;
}

export function useRoomMembers(slug: string, options: UseRoomMembersOptions = {}): UseRoomMembersResult {
  const { enabled = true } = options;
  const active = enabled && slug !== '';
  const [members, setMembers] = useState<APIRoomMember[]>([]);
  const [loading, setLoading] = useState(active);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!active) {
      // Keep the same empty list when there is nothing to forget, so nothing re-renders.
      setMembers((known) => (known.length === 0 ? known : []));
      setError(null);
      setLoading(false);
      return;
    }

    // An answer that arrives after the room changed or the read was disabled is dropped.
    let current = true;
    const fetchMembers = async () => {
      try {
        setLoading(true);
        const response = await api.getMembers(slug);
        if (!current) return;
        setMembers(response.data);
        setError(null);
      } catch (err) {
        if (!current) return;
        setError(err instanceof Error ? err : new Error('Failed to fetch members'));
        setMembers([]);
      } finally {
        if (current) setLoading(false);
      }
    };

    fetchMembers();
    return () => {
      current = false;
    };
  }, [slug, active]);

  return { members, loading, error };
}
