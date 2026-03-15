package billing

import "fmt"

// @func releaseFunds
// @description Calculates 10% platform fee and releases 90% to freelancer

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
		return ReleaseFundsResponse{}, fmt.Errorf("release amount must be positive: %d", req.Amount)
	}
	if req.FreelancerID == 0 {
		return ReleaseFundsResponse{}, fmt.Errorf("freelancer ID is required")
	}
	platformFee := req.Amount / 10
	payout := req.Amount - platformFee
	transactionID := req.GigID*10000 + payout
	return ReleaseFundsResponse{TransactionID: transactionID}, nil
}
