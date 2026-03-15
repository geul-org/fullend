CREATE TABLE actions (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    action_type VARCHAR(255) NOT NULL,
    payload_template TEXT NOT NULL DEFAULT '',
    sequence_order INTEGER NOT NULL
);
