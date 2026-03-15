package model

import (
	"encoding/json"
	"time"
)

type Action struct {
	ID           int64 `json:"id"`
	WorkflowID   int64 `json:"workflow_id"`
	ActionType   string `json:"action_type"`
	PayloadTemplate json.RawMessage `json:"payload_template"`
	SequenceOrder int64 `json:"sequence_order"`
}

type Execution struct {
	ID           int64 `json:"id"`
	WorkflowID   int64 `json:"workflow_id"`
	OrgID        int64 `json:"org_id"`
	LogStatus    string `json:"log_status"`
	CreditsSpent int64 `json:"credits_spent"`
	ExecutedAt   time.Time `json:"executed_at"`
}

type Organization struct {
	ID           int64 `json:"id"`
	Name         string `json:"name"`
	PlanType     string `json:"plan_type"`
	CreditsBalance int64 `json:"credits_balance"`
}

type User struct {
	ID           int64 `json:"id"`
	OrgID        int64 `json:"org_id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
}

type Workflow struct {
	ID           int64 `json:"id"`
	OrgID        int64 `json:"org_id"`
	Title        string `json:"title"`
	TriggerEvent string `json:"trigger_event"`
	Status       string `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
