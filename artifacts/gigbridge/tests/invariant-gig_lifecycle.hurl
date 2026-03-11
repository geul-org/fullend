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

HTTP 201

# PublishGig
PUT {{host}}/gigs/{{gigResult_gig.id}}/publish

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

HTTP 201

# AcceptProposal
POST {{host}}/proposals/{{proposalResult_proposal.id}}/accept

HTTP 200

# SubmitWork
POST {{host}}/gigs/{{gigResult_gig.id}}/submit-work

HTTP 200

# ApproveWork
POST {{host}}/gigs/{{gigResult_gig.id}}/approve

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

HTTP 201

# PublishGig
PUT {{host}}/gigs/{{gig2_gig.id}}/publish

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

HTTP 201

# AcceptProposal
POST {{host}}/proposals/{{proposal2_proposal.id}}/accept

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

# SubmitWork
POST {{host}}/gigs/{{gig2_gig.id}}/submit-work

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

HTTP 201

# PublishGig
PUT {{host}}/gigs/{{gig3_gig.id}}/publish

HTTP 200

# ApproveWork
POST {{host}}/gigs/{{gig3_gig.id}}/approve

HTTP 409

