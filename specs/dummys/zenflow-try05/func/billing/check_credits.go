package billing

import "errors"

// @func checkCredits
// @error 402
// @description Checks if organization has sufficient credits

type CheckCreditsRequest struct {
	Balance int64
}

type CheckCreditsResponse struct{}

func CheckCredits(req CheckCreditsRequest) (CheckCreditsResponse, error) {
	if req.Balance <= 0 {
		return CheckCreditsResponse{}, errors.New("insufficient credits")
	}
	return CheckCreditsResponse{}, nil
}
