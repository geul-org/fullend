package billing

// @func holdEscrow
// @description Simulates locking funds in escrow for a gig

type HoldEscrowRequest struct {
	GigID    int64
	Amount   int64
	ClientID int64
}

type HoldEscrowResponse struct {
	TransactionID int64
}

func HoldEscrow(req HoldEscrowRequest) (HoldEscrowResponse, error) {
	txID := req.GigID*1000 + req.Amount
	return HoldEscrowResponse{TransactionID: txID}, nil
}
