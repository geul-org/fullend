CREATE TABLE organizations (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    plan_type VARCHAR(50) NOT NULL CHECK (plan_type IN ('free', 'pro', 'enterprise')),
    credits_balance INTEGER NOT NULL DEFAULT 100
);
