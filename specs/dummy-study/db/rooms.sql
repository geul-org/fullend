CREATE TABLE rooms (
    id         BIGSERIAL    PRIMARY KEY,
    name       VARCHAR(50)  NOT NULL,
    capacity   INT          NOT NULL,
    location   VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
