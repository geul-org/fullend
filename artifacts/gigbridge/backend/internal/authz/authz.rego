package authz

# @ownership gig: gigs.client_id
# @ownership gig_assignee: gigs.freelancer_id
# @ownership proposal: proposals.freelancer_id

default allow = false

# CreateGig: Role 'client'
allow {
    input.action == "CreateGig"
    input.claims.role == "client"
}

# SubmitProposal: Role 'freelancer' AND not own gig
allow {
    input.action == "SubmitProposal"
    input.claims.role == "freelancer"
    input.claims.user_id != input.resource_owner_id
}

# AcceptProposal: Role 'client' AND owns gig
allow {
    input.action == "AcceptProposal"
    input.claims.role == "client"
    input.claims.user_id == input.resource_owner_id
}

# SubmitWork: Role 'freelancer' AND is gig_assignee
allow {
    input.action == "SubmitWork"
    input.claims.role == "freelancer"
    input.claims.user_id == input.resource_owner_id
}

# ApproveWork: Role 'client' AND owns gig
allow {
    input.action == "ApproveWork"
    input.claims.role == "client"
    input.claims.user_id == input.resource_owner_id
}
