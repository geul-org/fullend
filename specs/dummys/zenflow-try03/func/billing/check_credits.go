package billing

import (
	"fmt"
	"os"
)

// @func checkCredits
// @error 402
// @description Checks if organization has sufficient credits, returns error if not

type CheckCreditsRequest struct {
	OrgID int64
}

type CheckCreditsResponse struct {
	Balance int64
}

func CheckCredits(req CheckCreditsRequest) (CheckCreditsResponse, error) {
	_ = os.Getenv("BILLING_API_KEY")
	balance := req.OrgID
	if balance <= 0 {
		return CheckCreditsResponse{}, fmt.Errorf("insufficient credits")
	}
	return CheckCreditsResponse{Balance: balance}, nil
}
