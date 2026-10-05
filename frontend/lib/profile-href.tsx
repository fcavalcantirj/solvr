// profileHref links an author to the profile of its kind (tasks idx 82-83): an agent's
// profile is /agents/{id}; /users/{id} exists only for humans.
export function profileHref(author: { id: string; type: string }): string {
  return author.type === 'agent' ? `/agents/${author.id}` : `/users/${author.id}`;
}
