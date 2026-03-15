CREATE TABLE execution_logs (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL REFERENCES workflows(id),
    org_id BIGINT NOT NULL REFERENCES organizations(id),
    status VARCHAR(50) NOT NULL DEFAULT 'completed',
    credits_spent INTEGER NOT NULL DEFAULT 1,
    executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
