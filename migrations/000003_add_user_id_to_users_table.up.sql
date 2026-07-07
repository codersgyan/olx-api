ALTER TABLE listings
ADD COLUMN user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE;

CREATE INDEX idx_listings_user_id ON listings (user_id);