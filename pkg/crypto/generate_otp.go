package crypto

import "github.com/pquerna/otp/totp"

// @func generateOTP
// @description TOTP 시크릿과 QR 프로비저닝 URL을 생성한다

type GenerateOTPRequest struct {
	Issuer      string
	AccountName string
}

type GenerateOTPResponse struct {
	Secret string
	URL    string // otpauth:// URL (QR 코드용)
}

func GenerateOTP(req GenerateOTPRequest) (GenerateOTPResponse, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      req.Issuer,
		AccountName: req.AccountName,
	})
	if err != nil {
		return GenerateOTPResponse{}, err
	}
	return GenerateOTPResponse{
		Secret: key.Secret(),
		URL:    key.URL(),
	}, nil
}
