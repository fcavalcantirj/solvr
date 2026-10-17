-- A public room enters search engines, the sitemap, the public room list and the featured
-- rooms only when its owner lists it (SPEC.md 27.1). New rooms start unlisted; every room
-- that is public when this runs is listed, so no existing page changes.
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS listed BOOLEAN NOT NULL DEFAULT false;
UPDATE rooms SET listed = true WHERE deleted_at IS NULL AND is_private = false AND listed = false;
COMMENT ON COLUMN rooms.listed IS 'True when the owner lists a public room: only a listed public room is indexable, in the sitemap and in the public room list. New rooms start unlisted.';
