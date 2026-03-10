package service

// @get Gig gig = Gig.FindByID({ID: request.ID})
// @empty gig "Gig not found"
// @auth "publish" "gig" {id: gig.ID} "Not authorized"
// @state gig {status: gig.Status} "PublishGig" "Cannot publish gig"
// @put Gig.UpdateStatus({ID: gig.ID, Status: "open"})
// @get Gig gig = Gig.FindByID({ID: gig.ID})
// @response {
//   gig: gig
// }
func PublishGig() {}
