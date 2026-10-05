import type { APIPostsResponse } from '@/lib/api-types';
import { readForPage, UpstreamError } from './read-for-page';

// The posts a search engine may index, read on the server for the pages that link them
// (SPEC.md 27.2): the post archive (/posts/page/{n}) and the profile pages. The API decides
// which posts those are (GET /v1/posts?indexable=true applies the post sitemap's own rule);
// this file only asks.

// How many posts one archive page, and one profile's list, holds. The API's largest page.
export const ARCHIVE_PAGE_SIZE = 50;

// Only the canonical spelling of a page number is a page: /posts/page/01 is not /posts/page/1.
const PAGE_NUMBER = /^[1-9][0-9]{0,8}$/;

// archivePageNumber reads a page number from a URL segment, or null when the segment is not
// a positive integer written without a leading zero.
export function archivePageNumber(raw: string): number | null {
  return PAGE_NUMBER.test(raw) ? Number(raw) : null;
}

export interface IndexablePostsRead {
  page: number;
  // Only this author's posts (a profile page).
  author?: { type: 'agent' | 'human'; id: string };
}

// readIndexablePosts reads one page of the indexable posts, newest first. Any answer but a
// list fails the page with a retryable 5xx (lib/seo/read-for-page.ts): the list refuses
// nothing a page could turn into a 404, and a page rendered without its posts would take
// every link it carries away from them without a sign.
export async function readIndexablePosts({ page, author }: IndexablePostsRead): Promise<APIPostsResponse> {
  let path = `/v1/posts?indexable=true&sort=new&page=${page}&per_page=${ARCHIVE_PAGE_SIZE}`;
  if (author) path += `&author_type=${author.type}&author_id=${encodeURIComponent(author.id)}`;
  const { status, data } = await readForPage<APIPostsResponse>(path);
  if (!data || !Array.isArray(data.data) || !data.meta) {
    throw new UpstreamError(`${path}: API answered ${status} without a list`);
  }
  return data;
}
