-- Reverse the canonical Reply model (BART-585).

ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check
    CHECK (target_type IN ('post', 'answer', 'approach', 'response', 'comment'));

ALTER TABLE votes DROP CONSTRAINT IF EXISTS votes_target_type_check;
ALTER TABLE votes ADD CONSTRAINT votes_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'blog_post'));

DROP TABLE IF EXISTS replies;
