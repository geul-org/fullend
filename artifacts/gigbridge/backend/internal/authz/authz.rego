package authz

# @ownership gig: gigs.client_id
# @ownership gig_assignee: gigs.freelancer_id
# @ownership proposal: proposals.freelancer_id

default allow = false

# CreateGig: Role 'client'
allow if {
    input.action == "CreateGig"
    input.resource == "gig"
    input.claims.role == "client"
}

# PublishGig: Owner only
allow if {
    input.action == "PublishGig"
    input.resource == "gig"
    input.claims.user_id == input.resource_owner_id
}

# SubmitProposal: Role 'freelancer' AND not own gig
allow if {
    input.action == "SubmitProposal"
    input.resource == "gig"
    input.claims.role == "freelancer"
    input.claims.user_id != input.resource_owner_id
}

# AcceptProposal: Role 'client' AND owns gig
allow if {
    input.action == "AcceptProposal"
    input.resource == "gig"
    input.claims.role == "client"
    input.claims.user_id == input.resource_owner_id
}

# RejectProposal: Role 'client' AND owns gig
allow if {
    input.action == "RejectProposal"
    input.resource == "gig"
    input.claims.role == "client"
    input.claims.user_id == input.resource_owner_id
}

# SubmitWork: Role 'freelancer' AND is gig assignee
allow if {
    input.action == "SubmitWork"
    input.resource == "gig_assignee"
    input.claims.role == "freelancer"
    input.claims.user_id == input.resource_owner_id
}

# ApproveWork: Role 'client' AND owns gig
allow if {
    input.action == "ApproveWork"
    input.resource == "gig"
    input.claims.role == "client"
    input.claims.user_id == input.resource_owner_id
}

# RaiseDispute: Either client (owner) or freelancer (assignee)
allow if {
    input.action == "RaiseDispute"
    input.resource == "gig"
    input.claims.user_id == input.resource_owner_id
}
