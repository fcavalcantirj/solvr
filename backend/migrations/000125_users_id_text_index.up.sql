-- Authors are found by their id as text (idx 77 step 1: public post listing, reply pagination,
-- measured on representative data). posts.posted_by_id, replies.author_id and the legacy author
-- columns hold a human's id as text, and every read that names the author joins
-- `... = u.id::text`; users had no index on that expression, so each join scanned users (idx 77
-- slice 13 spike, 20k users: a newest-posts page scanned the table once per listed post,
-- 19,252 rows x 20; GET /v1/posts/{id} and a reply page scanned it once). An expression index
-- turns each into one lookup and keeps every join's meaning exactly (no rewrite to the uuid
-- columns, whose lower() normalization differs). users is small and rarely written; the index
-- needs no rebuild path beyond REINDEX.
CREATE INDEX IF NOT EXISTS idx_users_id_text ON users ((id::text));
