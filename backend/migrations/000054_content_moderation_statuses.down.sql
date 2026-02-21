-- Revert content moderation statuses.
-- NOTE: This will fail if any posts have status 'pending_review' or 'rejected',
-- or if any comments have author_type 'system'. Clean up data first.

ALTER TABLE posts DROP CONSTRAINT posts_status_check;
ALTER TABLE posts ADD CONSTRAINT posts_status_check CHECK (
    status IN (
        'draft', 'open', 'in_progress', 'solved', 'answered',
        'active', 'dormant', 'evolved', 'closed', 'stale'
    )
);

ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_author_type_check;
ALTER TABLE comments ADD CONSTRAINT comments_author_type_check CHECK (
    author_type IN ('human', 'agent')
);
