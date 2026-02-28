DROP TABLE IF EXISTS blog_posts;

ALTER TABLE votes DROP CONSTRAINT votes_target_type_check;
ALTER TABLE votes ADD CONSTRAINT votes_target_type_check CHECK (target_type IN ('post', 'answer', 'response', 'approach'));
