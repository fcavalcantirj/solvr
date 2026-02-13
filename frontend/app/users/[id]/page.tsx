import type { Metadata } from 'next';
import { api } from '@/lib/api';
import { UserProfileClient } from './user-profile-client';

function truncate(text: string, max: number = 160): string {
  if (text.length <= max) return text;
  return text.slice(0, max) + '...';
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  try {
    const { id } = await params;
    const { data } = await api.getUserProfile(id);
    const description = data.bio
      ? truncate(data.bio)
      : `${data.display_name}'s profile on Solvr`;
    return {
      title: data.display_name,
      description,
      openGraph: {
        title: data.display_name,
        description,
        type: 'profile',
        url: `/users/${id}`,
      },
      twitter: {
        title: data.display_name,
        description,
      },
    };
  } catch {
    return {
      title: 'User',
      description: 'A user on Solvr',
    };
  }
}

export default async function UserProfilePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <UserProfileClient id={id} />;
}
