package authz

# @ownership gig: gigs.client_id
# @ownership gig_assignee: gigs.freelancer_id
# @ownership proposal: proposals.freelancer_id

default allow = false

allow if {
    input.action == "PublishGig"
    input.resource == "gig"
    input.claims.role == "client"
    data.owners.gig[input.resource_id] == input.claims.user_id
}

allow if {
    input.action == "SubmitProposal"
    input.resource == "gig"
    input.claims.role == "freelancer"
    data.owners.gig[input.resource_id] != input.claims.user_id
}

allow if {
    input.action == "AcceptProposal"
    input.resource == "gig"
    input.claims.role == "client"
    data.owners.gig[input.resource_id] == input.claims.user_id
}

allow if {
    input.action == "RejectProposal"
    input.resource == "gig"
    input.claims.role == "client"
    data.owners.gig[input.resource_id] == input.claims.user_id
}

allow if {
    input.action == "SubmitWork"
    input.resource == "gig_assignee"
    input.claims.role == "freelancer"
    data.owners.gig_assignee[input.resource_id] == input.claims.user_id
}

allow if {
    input.action == "ApproveWork"
    input.resource == "gig"
    input.claims.role == "client"
    data.owners.gig[input.resource_id] == input.claims.user_id
}

allow if {
    input.action == "RaiseDispute"
    input.resource == "gig"
    input.claims.role == "client"
    data.owners.gig[input.resource_id] == input.claims.user_id
}
