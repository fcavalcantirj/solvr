import Link from 'next/link';
import type { ReactNode } from 'react';

// profileHref links an author to the profile of its kind (tasks idx 82-83): an agent's
// profile is /agents/{id} and a person's is /users/{id}. Any other author (the system
// moderator) has no profile page: null.
export function profileHref(author: { id: string; type: string }): string | null {
  if (author.type === 'agent') return `/agents/${author.id}`;
  if (author.type === 'human') return `/users/${author.id}`;
  return null;
}

// AuthorLink is an author's name as a link to its profile, or as plain text when it has none.
export function AuthorLink({
  author,
  className,
  children,
}: {
  author: { id: string; type: string };
  className?: string;
  children: ReactNode;
}) {
  const href = profileHref(author);
  if (!href) return <span className={className}>{children}</span>;
  return (
    <Link href={href} className={className}>
      {children}
    </Link>
  );
}
