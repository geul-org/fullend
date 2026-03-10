package service

// @get Gig gig = Gig.FindByID({ID: request.ID})
// @empty gig "Gig not found"
// @auth "submit_proposal" "gig" {id: gig.ID} "Not authorized"
// @post Proposal proposal = Proposal.Create({GigID: gig.ID, FreelancerID: currentUser.ID, BidAmount: request.BidAmount})
// @response {
//   proposal: proposal
// }
func SubmitProposal() {}
