-- +goose Up
CREATE TABLE trip_idempotency_keys (
    id UUID PRIMARY KEY,
    request_hash TEXT NOT NULL,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CREATE INDEX trip_idempotency_keys_id_idx
--     ON trip_idempotency_keys (created_at);
    
-- +goose Down
DROP TABLE trip_idempotency_keys;
