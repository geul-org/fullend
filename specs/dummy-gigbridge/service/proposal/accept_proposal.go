package service

import "github.com/gigbridge/api/internal/billing"

// @get Proposal proposal = Proposal.FindByID({ID: request.ID})
// @empty proposal "Proposal not found"
// @get Gig gig = Gig.FindByID({ID: proposal.GigID})
// @empty gig "Gig not found"
// @auth "accept" "gig" {id: gig.ID} "Not authorized"
// @state proposal {status: proposal.Status} "AcceptProposal" "Cannot accept proposal"
// @state gig {status: gig.Status} "AcceptProposal" "Cannot accept proposal"
// @put Proposal.UpdateStatus({ID: proposal.ID, Status: "accepted"})
// @put Gig.AssignFreelancer({ID: gig.ID, FreelancerID: proposal.FreelancerID})
// @put Gig.UpdateStatus({ID: gig.ID, Status: "in_progress"})
// @call int64 transactionID = billing.HoldEscrow({GigID: gig.ID, Amount: gig.Budget, ClientID: gig.ClientID})
// @get Gig gig = Gig.FindByID({ID: gig.ID})
// @response {
//   gig: gig,
//   transactionID: transactionID
// }
func AcceptProposal() {}
