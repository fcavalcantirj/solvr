import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import type { APIRoomMember } from '@/lib/api-types';

interface UseRoomMembersResult {
  members: APIRoomMember[];
  loading: boolean;
  error: Error | null;
}

export function useRoomMembers(slug: string): UseRoomMembersResult {
  const [members, setMembers] = useState<APIRoomMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    const fetchMembers = async () => {
      try {
        setLoading(true);
        const response = await api.getMembers(slug);
        setMembers(response.data);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err : new Error('Failed to fetch members'));
        setMembers([]);
      } finally {
        setLoading(false);
      }
    };

    fetchMembers();
  }, [slug]);

  return { members, loading, error };
}
