import { PageSection } from '@/components/page/page-section';
import { PostLinkList, type LinkedPost } from './post-link-list';

// An author's posts on that author's profile, as plain links in server HTML (SPEC.md 27.2).
// The profile's own lists load in the browser, which a crawler does not run, so without
// this section a profile linked none of its author's posts. The page reads the posts (the
// API's indexable list, filtered by author) and hands them here; nothing is fetched or
// decided in this file.

export function AuthorPosts({
  name,
  posts,
  total,
}: {
  // The author as the profile names them.
  name: string;
  // The newest of the author's indexable posts, as the API listed them.
  posts: LinkedPost[];
  // How many indexable posts the author has in all (the list's meta.total).
  total: number;
}) {
  if (posts.length === 0) return null;
  const count =
    total > posts.length
      ? `The newest ${posts.length} of ${total} posts.`
      : `${total} ${total === 1 ? 'post' : 'posts'}.`;
  return (
    <PageSection heading={`Posts by ${name}`} intro={count}>
      <PostLinkList posts={posts} testId="author-posts" />
    </PageSection>
  );
}
