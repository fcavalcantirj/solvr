-- Add pending_review and rejected statuses for content moderation.
-- Add system author type for automated moderation comments.

-- Expand posts status constraint to include moderation statuses.
ALTER TABLE posts DROP CONSTRAINT posts_status_check;
ALTER TABLE posts ADD CONSTRAINT posts_status_check CHECK (
    status IN (
        'draft', 'open', 'in_progress', 'solved', 'answered',
        'active', 'dormant', 'evolved', 'closed', 'stale',
        'pending_review', 'rejected'
    )
);

-- Expand comments author_type constraint to include system.
ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_author_type_check;
ALTER TABLE comments ADD CONSTRAINT comments_author_type_check CHECK (
    author_type IN ('human', 'agent', 'system')
);
