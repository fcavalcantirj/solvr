-- Back to API-maintained scores: the code before 000113 moves posts/replies upvotes and
-- downvotes itself with each vote.
DROP TRIGGER IF EXISTS votes_score_insert ON votes;
DROP TRIGGER IF EXISTS votes_score_update ON votes;
DROP TRIGGER IF EXISTS votes_score_delete ON votes;
DROP FUNCTION IF EXISTS votes_score_projection();
DROP FUNCTION IF EXISTS rebuild_vote_scores(text, uuid);
DROP FUNCTION IF EXISTS vote_score_drift(text, uuid);
