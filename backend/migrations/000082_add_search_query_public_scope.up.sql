-- Record whether a search ran over PUBLIC CONTENT ONLY.
--
-- GET /v1/search is family-scoped (see searchVisibilityClause): a signed-in
-- human, or an agent claimed by one, can match posts that are not public. The
-- term that found them is therefore not a public search term, and its text may
-- never be published on a public page.
--
-- NULL is meaningful and is the ONLY value historical rows can have: those
-- searches were served before this was recorded, so their eligibility is
-- genuinely unknown. They may be counted in explicitly labeled aggregates;
-- their query text is never published.
ALTER TABLE search_queries ADD COLUMN IF NOT EXISTS public_scope BOOLEAN;

COMMENT ON COLUMN search_queries.public_scope IS
    'TRUE: the search ran over public content only. FALSE: it ran family-scoped and could have matched protected content. NULL: recorded before scope was instrumented - eligibility unknown, count only, never publish the text.';

-- The homepage reads the publishable rows newest-first within a window.
CREATE INDEX IF NOT EXISTS idx_search_queries_public_scope
    ON search_queries (searched_at DESC)
    WHERE public_scope;
