package crypto

import (
	"fmt"

	"github.com/pquerna/otp/totp"
)

// @func verifyOTP
// @description TOTP 코드가 시크릿과 일치하는지 검증한다

type VerifyOTPRequest struct {
	Code   string
	Secret string
}

type VerifyOTPResponse struct{}

func VerifyOTP(req VerifyOTPRequest) (VerifyOTPResponse, error) {
	if !totp.Validate(req.Code, req.Secret) {
		return VerifyOTPResponse{}, fmt.Errorf("invalid OTP code")
	}
	return VerifyOTPResponse{}, nil
}
