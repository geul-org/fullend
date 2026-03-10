# Hurl 스모크 테스트 문제점 보고서

## 배경

dummy-lesson 프로젝트에 대해 `fullend gen`으로 코드 산출 후, 생성된 서버를 빌드하고 Hurl 스모크 테스트를 실행한 결과 발견된 문제점들.

## 1. fullend Hurl generator 버그 (수정 완료)

### 1-1. 응답 JSON 필드명 불일치
- **증상**: `jsonpath "$.user.ID" exists` 실패
- **원인**: Hurl generator가 OpenAPI 스키마 속성명(PascalCase: `ID`)을 그대로 사용하지만, 실제 서버 응답은 sqlc 모델의 snake_case JSON 태그(`json:"id"`)를 따름
- **수정**: `hurl_util.go`의 `generateResponseAssertions`, `inferCaptureField`에서 nested object 필드명을 snake_case로 변환
- **파일**: `artifacts/internal/gluegen/hurl_util.go`

### 1-2. 토큰 캡처 경로 불일치
- **증상**: `jsonpath "$.token.AccessToken"` 캡처 실패
- **원인**: OpenAPI 스키마는 `token`을 Token 객체로 정의하지만, SSaC codegen은 `out.AccessToken` 문자열을 직접 `"token"` 키로 wrap → 응답은 `{"token": "jwt-string"}`
- **수정**: `hurl.go`, `hurl_scenario.go`에서 `$.token.AccessToken` → `$.token`으로 변경
- **파일**: `artifacts/internal/gluegen/hurl.go`, `artifacts/internal/gluegen/hurl_scenario.go`

---

## 2. SSaC codegen 문제 (미수정 — SSaC 프로젝트 수정 필요)

### 2-1. import 경로 오류 — `"auth"` stdlib 충돌
- **증상**: `import "auth"` → Go 1.25에서 stdlib `auth` 패키지와 충돌하여 컴파일 실패
- **원인**: SSaC codegen의 `@func auth.hashPassword` 처리 시 import 경로를 패키지명만으로 생성
- **기대 동작**: `import "github.com/geul-org/fullend/pkg/auth"` 생성
- **영향**: auth 관련 call sequence가 있는 모든 서비스 파일
- **수동 패치**: `"auth"` → `"github.com/geul-org/fullend/pkg/auth"`

### 2-2. Request/Response 명명 미반영
- **증상**: `auth.HashPasswordInput` 등 컴파일 에러
- **원인**: Phase 26에서 pkg 파일들의 `*Input/*Output` → `*Request/*Response`로 변경했으나, SSaC codegen은 여전히 `*Input/*Output` 생성
- **기대 동작**: `auth.HashPasswordRequest{...}` 생성
- **영향**: `@sequence call`이 있는 모든 서비스 파일

### 2-3. IssueToken 필드명 불일치
- **증상**: `auth.IssueTokenInput{ID: user.ID}` 컴파일 에러
- **원인**: SSaC codegen이 `@param user.ID`에서 필드명 `ID`를 그대로 사용하지만, 실제 IssueTokenRequest 구조체는 `UserID` 필드를 가짐
- **기대 동작**: Request 구조체의 실제 필드명과 매칭
- **수동 패치**: `{ID: user.ID}` → `{UserID: user.ID}`

### 2-4. VerifyPassword 반환값 처리
- **증상**: `err = auth.VerifyPassword(...)` 컴파일 에러
- **원인**: SSaC codegen이 결과가 없는 call에 대해 `err =` 패턴을 생성하지만, 실제 함수는 `(VerifyPasswordResponse, error)` 반환
- **기대 동작**: `_, err =` 패턴 생성
- **수동 패치**: `err =` → `_, err =`

### 2-5. IssueToken 응답 필드명
- **증상**: `token := out.Token` — Token 필드 없음
- **원인**: SSaC codegen이 `@result token Token` 해석 시 `out.Token` 생성하지만, 실제 IssueTokenResponse는 `AccessToken` 필드를 가짐
- **기대 동작**: Response 구조체의 실제 필드명 사용
- **수동 패치**: `out.Token` → `out.AccessToken`

---

## 3. glue-gen 문제 (미수정 — fullend glue-gen 수정 필요)

### 3-1. DefaultCurrentUser JWT 미구현
- **증상**: 인증 필요 엔드포인트에서 항상 빈 CurrentUser 반환
- **원인**: glue-gen이 `DefaultCurrentUser`를 stub으로만 생성 (`return &model.CurrentUser{}`)
- **기대 동작**: Authorization 헤더에서 JWT 파싱 → `auth.VerifyToken` 호출 → CurrentUser 채움
- **수동 패치**: `auth.VerifyToken` 호출 코드 직접 작성

### 3-2. Authz 미연결
- **증상**: `h.Authz.Check()` 호출 시 nil pointer panic
- **원인**: glue-gen이 `main.go` 생성 시 handler에 `Authz` 필드를 설정하지 않음. SSaC가 handler struct에 `Authz model.Authorizer` 필드는 생성하지만, main.go 초기화 코드에는 누락
- **기대 동작**: `authz.New(conn)` 호출 후 모든 handler에 `Authz: az` 설정
- **수동 패치**: main.go에 import 추가 + `az, err := authz.New(conn)` + 각 handler에 `Authz: az` 추가

---

## 4. OpenAPI 스키마 ↔ 실제 응답 불일치 (설계 이슈)

### 4-1. 응답 필드 네이밍 컨벤션 불일치
- **OpenAPI**: PascalCase (`ID`, `Email`, `CreatedAt`)
- **sqlc 모델 JSON 태그**: snake_case (`id`, `email`, `created_at`)
- **영향**: OpenAPI 스키마 기반 클라이언트 코드가 실제 응답과 불일치
- **결정 필요**: 어느 쪽을 표준으로 할 것인가?

### 4-2. Token 응답 구조 불일치
- **OpenAPI**: `{"token": {"AccessToken": "jwt..."}}` (Token 객체)
- **실제 응답**: `{"token": "jwt..."}` (문자열 직접)
- **원인**: SSaC codegen이 `@result token Token` + `@var token`에서 `out.AccessToken`을 추출하여 문자열로 wrap

---

## 수정 우선순위

1. **SSaC codegen** (2-1 ~ 2-5): call sequence의 import 경로, struct 명명, 필드 매칭 — 이게 안 되면 생성된 코드가 컴파일 안 됨
2. **glue-gen** (3-1 ~ 3-2): JWT 파싱 + Authz 연결 — 이게 안 되면 인증/인가가 동작 안 함
3. **설계 결정** (4-1 ~ 4-2): JSON 네이밍 컨벤션 통일

## 테스트 환경

- dummy-lesson 프로젝트
- PostgreSQL: docker (dummy-lesson-pg, port 15432)
- Go 1.25, fullend Phase 26 완료 상태
