package billing

// @func releaseFunds
// @description Release funds to freelancer with 10 percent platform fee deduction

type ReleaseFundsRequest struct {
	GigID        int64
	Amount       int64
	FreelancerID int64
}

type ReleaseFundsResponse struct {
	TransactionID int64
}

func ReleaseFunds(req ReleaseFundsRequest) (ReleaseFundsResponse, error) {
	// TODO: implement
	return ReleaseFundsResponse{}, nil
}
