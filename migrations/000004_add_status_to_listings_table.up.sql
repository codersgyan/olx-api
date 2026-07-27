ALTER TABLE listings
ADD COLUMN status TEXT NOT NULL DEFAULT 'ready' CHECK (
    status IN (
        'processing',
        'ready',
        'rejected'
    )
);

ALTER TABLE listings ALTER COLUMN status SET DEFAULT 'processing';

CREATE INDEX idx_listings_status ON listings (status);