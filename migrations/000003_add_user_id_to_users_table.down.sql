DROP INDEX IF EXISTS idx_listings_user_id ON listings (user_id);

ALTER TABLE listings DROP COLUMN user_id;