# Invariant: Organization with 0 credits cannot activate a workflow → 402 Payment Required

# Register admin in org with 0 credits (org_id=3)
POST {{host}}/auth/register
Content-Type: application/json
{
    "org_id": 3,
    "email": "broke@org.test",
    "password": "password123",
    "role": "admin"
}
HTTP 201

# Login
POST {{host}}/auth/login
Content-Type: application/json
{
    "email": "broke@org.test",
    "password": "password123"
}
HTTP 200
[Captures]
broke_token: jsonpath "$.access_token"

# Create workflow
POST {{host}}/workflows
Content-Type: application/json
Authorization: Bearer {{broke_token}}
{
    "title": "Test Workflow",
    "trigger_event": "test.event"
}
HTTP 201
[Captures]
broke_workflow_id: jsonpath "$.workflow.id"

# Try to activate → should fail with 402
POST {{host}}/workflows/{{broke_workflow_id}}/activate
Content-Type: application/json
Authorization: Bearer {{broke_token}}
HTTP 402
