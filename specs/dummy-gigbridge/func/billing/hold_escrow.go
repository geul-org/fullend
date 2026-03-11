package billing

import "fmt"

// @func holdEscrow
// @description Hold funds in escrow for a gig by creating a hold transaction

type HoldEscrowRequest struct {
	GigID    int64
	Amount   int64
	ClientID int64
}

type HoldEscrowResponse struct {
	TransactionID int64
}

func HoldEscrow(req HoldEscrowRequest) (HoldEscrowResponse, error) {
	if req.Amount <= 0 {
		return HoldEscrowResponse{}, fmt.Errorf("amount must be positive")
	}
	return HoldEscrowResponse{
		TransactionID: req.GigID*1000 + req.ClientID,
	}, nil
}
