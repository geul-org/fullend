CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,
    gig_id BIGINT NOT NULL REFERENCES gigs(id),
    tx_type VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
