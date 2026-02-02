/**
 * Post detail page layout with dynamic metadata
 * Per SPEC.md Part 19.2 SEO requirements
 */

export { generateMetadata } from './metadata';

export default function PostLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
