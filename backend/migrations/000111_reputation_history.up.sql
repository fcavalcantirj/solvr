-- Task idx 76 steps 3-4 (feature:leaderboards, feature:reputation): earned reputation is kept
-- as history across the contribution cutover.
--
-- The legacy reputation formula scores rows the canonical model does not keep in scoreable
-- form: solved and contributed problems, ideas, answers, accepted answers, responses and
-- comments (post types, statuses and contribution tables that the cleanup removes), and votes
-- on answers and responses (retargeted to replies by the cutover). RemapLegacyRelations
-- freezes one row per such earning event here, before it retargets any vote, so the points
-- stay exactly what the legacy rules gave. Votes on approaches earned nothing under those
-- rules; they are recorded with 0 points so their retargeted rows are never scored later.
--
-- Canonical readers add live points only for activity outside this history: confirmed votes
-- on posts and on replies whose vote id is not recorded here.
CREATE TABLE reputation_history (
    -- What earned the points, and the legacy row that earned them (a post, answer, response,
    -- comment or vote id). A post can earn twice (problem_contributed and problem_solved).
    source VARCHAR(30) NOT NULL CHECK (source IN (
        'problem_solved', 'problem_contributed', 'idea_posted',
        'answer_given', 'answer_accepted', 'response_given', 'comment_given',
        'answer_upvote', 'answer_downvote', 'response_upvote', 'response_downvote',
        'approach_vote')),
    source_id UUID NOT NULL,

    -- Who earned them (the author of the post or contribution, or of the voted contribution).
    owner_type VARCHAR(10) NOT NULL CHECK (owner_type IN ('agent', 'human')),
    owner_id VARCHAR(255) NOT NULL,

    -- The post the event belongs to, for per-tag leaderboards. NULL for comments, and once
    -- the post is hard-deleted: the earned points stay history.
    post_id UUID REFERENCES posts(id) ON DELETE SET NULL,

    points INTEGER NOT NULL,

    -- When it was earned: the legacy row's created_at (NULL when that row had none, which a
    -- time-windowed leaderboard never counts, as before).
    earned_at TIMESTAMPTZ,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (source, source_id)
);

-- A vote is frozen once, whatever its direction.
CREATE UNIQUE INDEX idx_reputation_history_vote ON reputation_history (source_id)
    WHERE source IN ('answer_upvote', 'answer_downvote', 'response_upvote', 'response_downvote', 'approach_vote');

CREATE INDEX idx_reputation_history_owner ON reputation_history (owner_type, owner_id, earned_at);

CREATE INDEX idx_reputation_history_post ON reputation_history (post_id) WHERE post_id IS NOT NULL;

COMMENT ON TABLE reputation_history IS 'Reputation earned under the legacy rules, frozen at the contribution cutover (task idx 76); canonical readers add live vote points outside it';
