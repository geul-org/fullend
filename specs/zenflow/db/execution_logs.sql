CREATE TABLE execution_logs (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL REFERENCES workflows(id),
    org_id BIGINT NOT NULL REFERENCES organizations(id),
    status VARCHAR(50) NOT NULL,
    credits_spent INTEGER NOT NULL DEFAULT 1,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_execution_logs_workflow_id ON execution_logs(workflow_id);
CREATE INDEX idx_execution_logs_org_id ON execution_logs(org_id);
