CREATE INDEX listings_ready_pagination_idx ON listings (created_at DESC, id DESC)
WHERE
    status = 'ready'