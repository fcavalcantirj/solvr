-- Partial covering indexes for counting non-deleted answers/approaches per post.
-- The existing idx_answers_question and idx_approaches_problem do NOT filter
-- on deleted_at, forcing the planner to fetch and filter rows.
-- These pre-filter deleted rows so the planner only touches live rows.

CREATE INDEX IF NOT EXISTS idx_answers_question_not_deleted
    ON answers(question_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_approaches_problem_not_deleted
    ON approaches(problem_id) WHERE deleted_at IS NULL;
