"use client";

import { useAuth } from "@/hooks/use-auth";
import { useShareVisit } from "@/hooks/use-share-visit";
import { useViewTracking } from "@/hooks/use-view-tracking";

// PostPageSignals tells the API what opening a post page means, and draws nothing.
//
// share_visit (idx 88): a post opened through a share link (?via=share) is counted once
// per browser tab and the marker then leaves the address bar. The API attributes the
// visit to the post only while the post is public.
//
// A view (POST /v1/posts/{id}/view): once per browser session per post. The API counts a
// signed-in reader under their own account and an anonymous one under this tab's session
// id, so the view waits until the stored session is read: sent earlier, a signed-in
// reader would go out before their credential is attached.
export function PostPageSignals({ postId }: { postId: string }) {
  const { isLoading } = useAuth();

  useShareVisit({ kind: "post", ref: postId }, "post_page");
  useViewTracking(postId, 0, { enabled: !isLoading });

  return null;
}
