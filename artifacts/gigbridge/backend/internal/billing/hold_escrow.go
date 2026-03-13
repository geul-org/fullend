package billing

import "fmt"

// @func holdEscrow
// @description Simulates locking funds in escrow

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
		return HoldEscrowResponse{}, fmt.Errorf("hold amount must be positive: %d", req.Amount)
	}
	if req.GigID == 0 {
		return HoldEscrowResponse{}, fmt.Errorf("gig ID is required")
	}
	if req.ClientID == 0 {
		return HoldEscrowResponse{}, fmt.Errorf("client ID is required")
	}
	transactionID := req.GigID*10000 + req.ClientID
	return HoldEscrowResponse{TransactionID: transactionID}, nil
}
