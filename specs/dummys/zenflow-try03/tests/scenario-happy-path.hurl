# Happy Path: Admin creates workflow → adds 2 actions → activates → executes → verify log and credit deduction

# Step 1: Register admin user
POST {{host}}/auth/register
Content-Type: application/json
{
    "org_id": 1,
    "email": "admin@zenflow.test",
    "password": "password123",
    "role": "admin"
}
HTTP 201
[Captures]
admin_id: jsonpath "$.user.id"

# Step 2: Login
POST {{host}}/auth/login
Content-Type: application/json
{
    "email": "admin@zenflow.test",
    "password": "password123"
}
HTTP 200
[Captures]
token: jsonpath "$.access_token"

# Step 3: Create workflow
POST {{host}}/workflows
Content-Type: application/json
Authorization: Bearer {{token}}
{
    "title": "Email Onboarding",
    "trigger_event": "user.signup"
}
HTTP 201
[Captures]
workflow_id: jsonpath "$.workflow.id"

# Step 4: Add action 1
POST {{host}}/workflows/{{workflow_id}}/actions
Content-Type: application/json
Authorization: Bearer {{token}}
{
    "action_type": "send_email",
    "payload_template": {"subject": "Welcome!"},
    "sequence_order": 1
}
HTTP 201

# Step 5: Add action 2
POST {{host}}/workflows/{{workflow_id}}/actions
Content-Type: application/json
Authorization: Bearer {{token}}
{
    "action_type": "send_email",
    "payload_template": {"subject": "Getting Started"},
    "sequence_order": 2
}
HTTP 201

# Step 6: Activate workflow
POST {{host}}/workflows/{{workflow_id}}/activate
Content-Type: application/json
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.workflow.status" == "active"

# Step 7: Execute workflow
POST {{host}}/workflows/{{workflow_id}}/execute
Content-Type: application/json
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.log.log_status" == "completed"
jsonpath "$.log.credits_spent" == 1
