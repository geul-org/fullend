# Invariant: Org A user cannot execute Org B's workflow → 403 Forbidden

# Step 1: Register Org 1 admin
POST {{host}}/auth/register
Content-Type: application/json
{
    "org_id": 1,
    "email": "admin@orga.test",
    "password": "password123",
    "role": "admin"
}
HTTP 201

# Login Org 1 admin
POST {{host}}/auth/login
Content-Type: application/json
{
    "email": "admin@orga.test",
    "password": "password123"
}
HTTP 200
[Captures]
org1_token: jsonpath "$.access_token"

# Create workflow in Org 1
POST {{host}}/workflows
Content-Type: application/json
Authorization: Bearer {{org1_token}}
{
    "title": "Org A Workflow",
    "trigger_event": "test.event"
}
HTTP 201
[Captures]
org1_workflow_id: jsonpath "$.workflow.id"

# Step 2: Register Org 2 user
POST {{host}}/auth/register
Content-Type: application/json
{
    "org_id": 2,
    "email": "other@orgb.test",
    "password": "password123",
    "role": "admin"
}
HTTP 201

# Login as Org 2 user
POST {{host}}/auth/login
Content-Type: application/json
{
    "email": "other@orgb.test",
    "password": "password123"
}
HTTP 200
[Captures]
other_token: jsonpath "$.access_token"

# Try to execute Org 1's workflow → should be forbidden or not found
POST {{host}}/workflows/{{org1_workflow_id}}/execute
Content-Type: application/json
Authorization: Bearer {{other_token}}
HTTP 409
