package service

// @get Proposal proposal = Proposal.FindByID({ID: request.ID})
// @empty proposal "Proposal not found"
// @get Gig gig = Gig.FindByID({ID: proposal.GigID})
// @empty gig "Gig not found"
// @auth "reject" "gig" {id: gig.ID} "Not authorized"
// @state proposal {status: proposal.Status} "RejectProposal" "Cannot reject proposal"
// @put Proposal.UpdateStatus({ID: proposal.ID, Status: "rejected"})
// @get Proposal proposal = Proposal.FindByID({ID: proposal.ID})
// @response {
//   proposal: proposal
// }
func RejectProposal() {}
