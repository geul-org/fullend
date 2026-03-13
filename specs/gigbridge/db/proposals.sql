CREATE TABLE proposals (
    id BIGSERIAL PRIMARY KEY,
    gig_id BIGINT NOT NULL REFERENCES gigs(id),
    freelancer_id BIGINT NOT NULL REFERENCES users(id),
    bid_amount BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending'
);
