# Auto-generated from gig_lifecycle.feature — do not edit.

# === Happy Path - Gig creation to fund release ===

# Register
POST {{host}}/auth/register
Content-Type: application/json
{
  "email": "gig_lifecycle-client@test.com",
  "password": "pass123",
  "role": "client",
  "name": "Client A"
}

HTTP 200

# Login
POST {{host}}/auth/login
Content-Type: application/json
{
  "email": "gig_lifecycle-client@test.com",
  "password": "pass123"
}

HTTP 200
[Captures]
clientToken: jsonpath "$.token"

# CreateGig
POST {{host}}/gigs
Authorization: Bearer {{clientToken}}
Content-Type: application/json
{
  "title": "Build API",
  "description": "REST API project",
  "budget": 1000
}

HTTP 200
[Captures]
gigResult_gig.id: jsonpath "$.gig.id"

# PublishGig
PUT {{host}}/gigs/{{gigResult_gig.id}}/publish
Authorization: Bearer {{clientToken}}

HTTP 200

# Register
POST {{host}}/auth/register
Content-Type: application/json
{
  "email": "gig_lifecycle-freelancer@test.com",
  "password": "pass123",
  "role": "freelancer",
  "name": "Freelancer A"
}

HTTP 200

# Login
POST {{host}}/auth/login
Content-Type: application/json
{
  "email": "gig_lifecycle-freelancer@test.com",
  "password": "pass123"
}

HTTP 200
[Captures]
freelancerToken: jsonpath "$.token"

# SubmitProposal
POST {{host}}/gigs/{{gigResult_gig.id}}/proposals
Authorization: Bearer {{freelancerToken}}
Content-Type: application/json
{
  "bid_amount": 900
}

HTTP 200
[Captures]
proposalResult_proposal.id: jsonpath "$.proposal.id"

# AcceptProposal
POST {{host}}/proposals/{{proposalResult_proposal.id}}/accept
Authorization: Bearer {{freelancerToken}}

HTTP 200

# SubmitWork
POST {{host}}/gigs/{{gigResult_gig.id}}/submit-work
Authorization: Bearer {{freelancerToken}}

HTTP 200

# ApproveWork
POST {{host}}/gigs/{{gigResult_gig.id}}/approve
Authorization: Bearer {{freelancerToken}}

HTTP 200


# === Unauthorized - Freelancer B cannot submit work on Freelancer A's gig ===

# Register
POST {{host}}/auth/register
Content-Type: application/json
{
  "email": "gig_lifecycle-client2@test.com",
  "password": "pass123",
  "role": "client",
  "name": "Client B"
}

HTTP 200

# Login
POST {{host}}/auth/login
Content-Type: application/json
{
  "email": "gig_lifecycle-client2@test.com",
  "password": "pass123"
}

HTTP 200
[Captures]
clientToken2: jsonpath "$.token"

# CreateGig
POST {{host}}/gigs
Authorization: Bearer {{clientToken2}}
Content-Type: application/json
{
  "title": "Design Logo",
  "description": "Logo design",
  "budget": 500
}

HTTP 200
[Captures]
gig2_gig.id: jsonpath "$.gig.id"

# PublishGig
PUT {{host}}/gigs/{{gig2_gig.id}}/publish
Authorization: Bearer {{clientToken2}}

HTTP 200

# Register
POST {{host}}/auth/register
Content-Type: application/json
{
  "email": "gig_lifecycle-freelancerA@test.com",
  "password": "pass123",
  "role": "freelancer",
  "name": "Freelancer A2"
}

HTTP 200

# Login
POST {{host}}/auth/login
Content-Type: application/json
{
  "email": "gig_lifecycle-freelancerA@test.com",
  "password": "pass123"
}

HTTP 200
[Captures]
flTokenA: jsonpath "$.token"

# SubmitProposal
POST {{host}}/gigs/{{gig2_gig.id}}/proposals
Authorization: Bearer {{flTokenA}}
Content-Type: application/json
{
  "bid_amount": 400
}

HTTP 200
[Captures]
proposal2_proposal.id: jsonpath "$.proposal.id"

# AcceptProposal
POST {{host}}/proposals/{{proposal2_proposal.id}}/accept
Authorization: Bearer {{flTokenA}}

HTTP 200

# Register
POST {{host}}/auth/register
Content-Type: application/json
{
  "email": "gig_lifecycle-freelancerB@test.com",
  "password": "pass123",
  "role": "freelancer",
  "name": "Freelancer B"
}

HTTP 200

# Login
POST {{host}}/auth/login
Content-Type: application/json
{
  "email": "gig_lifecycle-freelancerB@test.com",
  "password": "pass123"
}

HTTP 200
[Captures]
flTokenB: jsonpath "$.token"

# SubmitWork
POST {{host}}/gigs/{{gig2_gig.id}}/submit-work
Authorization: Bearer {{flTokenB}}

HTTP 403


# === Invalid State - Cannot approve work when gig is open ===

# Register
POST {{host}}/auth/register
Content-Type: application/json
{
  "email": "gig_lifecycle-client3@test.com",
  "password": "pass123",
  "role": "client",
  "name": "Client C"
}

HTTP 200

# Login
POST {{host}}/auth/login
Content-Type: application/json
{
  "email": "gig_lifecycle-client3@test.com",
  "password": "pass123"
}

HTTP 200
[Captures]
clientToken3: jsonpath "$.token"

# CreateGig
POST {{host}}/gigs
Authorization: Bearer {{clientToken3}}
Content-Type: application/json
{
  "title": "Write Docs",
  "description": "Documentation",
  "budget": 300
}

HTTP 200
[Captures]
gig3_gig.id: jsonpath "$.gig.id"

# PublishGig
PUT {{host}}/gigs/{{gig3_gig.id}}/publish
Authorization: Bearer {{clientToken3}}

HTTP 200

# ApproveWork
POST {{host}}/gigs/{{gig3_gig.id}}/approve
Authorization: Bearer {{clientToken3}}

HTTP 409

