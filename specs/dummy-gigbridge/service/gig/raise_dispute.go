package service

// @get Gig gig = Gig.FindByID({ID: request.ID})
// @empty gig "Gig not found"
// @auth "dispute" "gig" {id: gig.ID} "Not authorized"
// @state gig {status: gig.Status} "RaiseDispute" "Cannot raise dispute"
// @put Gig.UpdateStatus({ID: gig.ID, Status: "disputed"})
// @get Gig gig = Gig.FindByID({ID: gig.ID})
// @response {
//   gig: gig
// }
func RaiseDispute() {}
