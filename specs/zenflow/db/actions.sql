CREATE TABLE actions (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    action_type VARCHAR(255) NOT NULL,
    payload_template JSONB,
    sequence_order INTEGER NOT NULL
);

CREATE INDEX idx_actions_workflow_id ON actions(workflow_id);
