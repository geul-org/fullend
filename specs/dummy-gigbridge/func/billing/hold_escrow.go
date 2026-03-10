package billing

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
	// TODO: implement
	return HoldEscrowResponse{}, nil
}
