package service

import "github.com/gigbridge/api/internal/billing"

// @get Gig gig = Gig.FindByID({ID: request.ID})
// @empty gig "Gig not found"
// @auth "approve" "gig" {id: gig.ID} "Not authorized"
// @state gig {status: gig.Status} "ApproveWork" "Cannot approve work"
// @put Gig.UpdateStatus({ID: gig.ID, Status: "completed"})
// @call int64 transactionID = billing.ReleaseFunds({GigID: gig.ID, Amount: gig.Budget, FreelancerID: gig.FreelancerID})
// @get Gig gig = Gig.FindByID({ID: gig.ID})
// @response {
//   gig: gig,
//   transactionID: transactionID
// }
func ApproveWork() {}
