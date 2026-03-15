CREATE TABLE executions (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL REFERENCES workflows(id),
    org_id BIGINT NOT NULL REFERENCES organizations(id),
    log_status TEXT NOT NULL,
    credits_spent INTEGER NOT NULL DEFAULT 0,
    executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_executions_workflow_id ON executions(workflow_id);
CREATE INDEX idx_executions_org_id ON executions(org_id);
