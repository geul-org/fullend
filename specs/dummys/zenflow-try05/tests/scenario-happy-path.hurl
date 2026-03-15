# Happy Path: Register → Create Workflow → Add Actions → Activate → Execute → Check

# 1. Register admin user
POST http://localhost:8080/auth/register
Content-Type: application/json
{
    "org_name": "Acme Corp",
    "email": "admin@acme.com",
    "password": "TestPass123"
}
HTTP 200
[Captures]
token: jsonpath "$.access_token"

# 2. Create workflow
POST http://localhost:8080/workflows
Content-Type: application/json
Authorization: Bearer {{token}}
{
    "title": "Onboarding Flow",
    "trigger_event": "user_signup"
}
HTTP 200
[Captures]
workflow_id: jsonpath "$.workflow.id"

# 3. Add action 1
POST http://localhost:8080/workflows/{{workflow_id}}/actions
Content-Type: application/json
Authorization: Bearer {{token}}
{
    "action_type": "send_email",
    "payload_template": "welcome",
    "sequence_order": 1
}
HTTP 200

# 4. Add action 2
POST http://localhost:8080/workflows/{{workflow_id}}/actions
Content-Type: application/json
Authorization: Bearer {{token}}
{
    "action_type": "create_task",
    "payload_template": "onboarding_task",
    "sequence_order": 2
}
HTTP 200

# 5. Activate workflow
POST http://localhost:8080/workflows/{{workflow_id}}/activate
Content-Type: application/json
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.workflow.status" == "active"

# 6. Execute workflow
POST http://localhost:8080/workflows/{{workflow_id}}/execute
Content-Type: application/json
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.execution_log.status" == "completed"
jsonpath "$.execution_log.credits_spent" == 1

# 7. Check execution logs
GET http://localhost:8080/execution-logs
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.execution_logs" count == 1
