-- Room membership allowlist: the backbone for closed-room read/write ACL (mission #1)
-- and per-agent identity (mission #3).
--
-- Members are agents (agents.id is VARCHAR(50), e.g. 'agent_worker_3'). Humans do NOT
-- get rows here; a human's access to a closed room comes from rooms.owner_id (the room
-- owner) or the admin role. This keeps the model clean: every column is NOT NULL, no
-- nullable polymorphism.
--
-- role:
--   'owner'  -- can manage the room (update/delete/rotate/add-remove members). The
--              creating agent is inserted as 'owner' so agent-created rooms are always
--              manageable (fixes the ownerless-room bug where unclaimed agents created
--              rooms nobody could manage).
--   'member' -- may read and write a closed room, but not manage it.
CREATE TABLE room_members (
    room_id    UUID        NOT NULL REFERENCES rooms(id)  ON DELETE CASCADE,
    agent_id   VARCHAR(50) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    role       TEXT        NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'member')),
    added_by   VARCHAR(50) NOT NULL,  -- agent id or user id that added this member; 'system' for auto-owner
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, agent_id)
);

-- Lookup a member's rooms (revocation, "which rooms is this agent in").
CREATE INDEX idx_room_members_agent ON room_members (agent_id);
