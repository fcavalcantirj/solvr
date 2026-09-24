-- Retire the shared room bearer token (solvr_rm_..., rooms.token_hash).
--
-- room_members is the only room authority (000095-000097). Every /r/{slug}/* caller now
-- holds its OWN per-agent token (room_agent_tokens, solvr_rt_...) issued by
-- POST /v1/rooms/{slug}/handshake to an admitted member or family agent, so the shared
-- token no longer grants anything: room creation stops returning it, the rotate-token
-- route is gone, and the handshake no longer accepts it as a bootstrap credential.

ALTER TABLE rooms DROP COLUMN IF EXISTS token_hash;
