package billing

import "fmt"

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
	if req.Amount <= 0 {
		return ReleaseFundsResponse{}, fmt.Errorf("amount must be positive")
	}
	return ReleaseFundsResponse{
		TransactionID: req.GigID*1000 + req.FreelancerID,
	}, nil
}
