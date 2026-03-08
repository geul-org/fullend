package model

// Token은 로그인 세션 토큰이다.
type Token struct {
	AccessToken string
	ExpiresAt   string
}

// Session은 인증 세션 관리 모델이다.
// SSaC에서 @model Session.Create로 참조된다.
type Session interface {
	Create(userID int64) (Token, error)
}
