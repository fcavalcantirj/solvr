-- Add resolved_by and resolved_at to flags table to align with SPEC naming.
-- The original migration 000013 used reviewed_by/reviewed_at; the canonical names are resolved_by/resolved_at.
ALTER TABLE flags ADD COLUMN IF NOT EXISTS resolved_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE flags ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;
