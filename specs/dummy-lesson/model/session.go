package model

// @dto
// Token은 인증 토큰이다 (DDL 테이블 없음).
type Token struct {
	AccessToken string
	ExpiresAt   string
}

// @dto
// IssueTokenResponse는 JWT 토큰 발급 응답이다 (DDL 테이블 없음).
type IssueTokenResponse struct {
	AccessToken string
}

// @dto
// HashPasswordResponse는 비밀번호 해싱 응답이다 (DDL 테이블 없음).
type HashPasswordResponse struct {
	HashedPassword string
}
