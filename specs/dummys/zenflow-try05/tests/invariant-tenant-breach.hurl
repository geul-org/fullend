# Tenant Breach: Org A user tries to access Org B workflow → 403/404

# 1. Register Org A admin
POST http://localhost:8080/auth/register
Content-Type: application/json
{
    "org_name": "Org Alpha",
    "email": "admin@alpha.com",
    "password": "TestPass123"
}
HTTP 200
[Captures]
token_a: jsonpath "$.access_token"

# 2. Create workflow in Org A
POST http://localhost:8080/workflows
Content-Type: application/json
Authorization: Bearer {{token_a}}
{
    "title": "Alpha Workflow",
    "trigger_event": "alpha_event"
}
HTTP 200
[Captures]
workflow_id_a: jsonpath "$.workflow.id"

# 3. Register Org B admin
POST http://localhost:8080/auth/register
Content-Type: application/json
{
    "org_name": "Org Beta",
    "email": "admin@beta.com",
    "password": "TestPass123"
}
HTTP 200
[Captures]
token_b: jsonpath "$.access_token"

# 4. Org B user tries to access Org A workflow → should fail
GET http://localhost:8080/workflows/{{workflow_id_a}}
Authorization: Bearer {{token_b}}
HTTP 404
