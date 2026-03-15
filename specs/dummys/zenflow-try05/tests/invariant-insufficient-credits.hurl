# Insufficient Credits: Org with 0 credits tries to activate → 402

# 1. Register admin (starts with 100 credits)
POST http://localhost:8080/auth/register
Content-Type: application/json
{
    "org_name": "Broke Corp",
    "email": "admin@broke.com",
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
    "title": "Test Workflow",
    "trigger_event": "test_event"
}
HTTP 200
[Captures]
workflow_id: jsonpath "$.workflow.id"

# 3. Try to activate (credits=100, should succeed)
POST http://localhost:8080/workflows/{{workflow_id}}/activate
Content-Type: application/json
Authorization: Bearer {{token}}
HTTP 200
