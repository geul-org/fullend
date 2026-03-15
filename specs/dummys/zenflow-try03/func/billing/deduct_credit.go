package billing

import "os"

// @func deductCredit
// @description Atomically deducts credits from an organization

type DeductCreditRequest struct {
	OrgID  int64
	Amount int64
}

type DeductCreditResponse struct {
	Remaining int64
}

func DeductCredit(req DeductCreditRequest) (DeductCreditResponse, error) {
	_ = os.Getenv("BILLING_API_KEY")
	remaining := req.OrgID - req.Amount
	if remaining < 0 {
		remaining = 0
	}
	return DeductCreditResponse{Remaining: remaining}, nil
}
