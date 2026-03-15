CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL REFERENCES organizations(id),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL, -- @sensitive
    role TEXT NOT NULL CHECK (role IN ('admin', 'member'))
);
