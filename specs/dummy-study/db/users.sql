CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name          VARCHAR(100) NOT NULL,
    role          VARCHAR(20)  NOT NULL DEFAULT 'user',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
