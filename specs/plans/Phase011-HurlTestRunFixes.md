✅ 완료

# Phase 011: Scenario Hurl 실행 시 실패 수정

## 목표

`hurl --test`로 생성된 scenario/invariant hurl 파일 전체를 순차 실행했을 때 통과하도록 2가지 버그를 수정한다.

## 배경

Phase 010에서 hurl 생성 버그(리터럴 path param, assertion subField 케이스)를 수정한 뒤, 실제 서버에 대해 hurl 테스트를 실행하면 3개 파일이 실패한다:

```
tests/smoke.hurl:                    ✓ (17 requests)
tests/scenario-course-lifecycle.hurl: ✓ (6 requests)
tests/scenario-negative-auth.hurl:    ✗ POST /courses/1/enroll → 404 (expected 401)
tests/scenario-student-enrollment.hurl: ✗ POST /register → 500 (duplicate email)
tests/invariant-course-deletion.hurl:   ✗ POST /register → 500 (duplicate email)
```

### 버그 A: BearerAuth 미들웨어가 401을 반환하지 않음

생성된 `internal/middleware/bearerauth.go`가 토큰 없는 요청도 빈 `currentUser`를 설정하고 `c.Next()`로 통과시킨다. auth 그룹에 등록된 엔드포인트는 인증 필수이므로, 토큰이 없거나 유효하지 않으면 401을 반환해야 한다.

현재:
```go
if !strings.HasPrefix(header, "Bearer ") {
    c.Set("currentUser", &model.CurrentUser{})
    c.Next()   // ← 그냥 통과
    return
}
```

기대:
```go
if !strings.HasPrefix(header, "Bearer ") {
    c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
    return
}
```

이로 인해 negative-auth 시나리오에서 401 대신 OPA 통과 → handler 실행 → 500/404 발생.

### 버그 B: hurl 파일 간 이메일 충돌

Gherkin feature 파일들이 동일한 이메일을 사용한다:
- `course-lifecycle.feature`: `inst@test.com`
- `student-enrollment.feature`: `inst@test.com`, `student@test.com`
- `course-deletion.feature`: `inst@test.com`

hurl 파일이 순차 실행되면 첫 번째 `course-lifecycle.hurl`에서 `inst@test.com`이 등록되고, 이후 파일에서 같은 이메일로 재등록 시 unique constraint violation → 500.

해결: scenario-gen이 hurl을 생성할 때 이메일 리터럴에 feature 파일명 기반 prefix를 붙여 고유화한다.

## 변경 사항

### 1. `middlewaregen.go` — 토큰 없으면 401 Abort

```go
src := fmt.Sprintf(`package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/geul-org/fullend/pkg/auth"
	"%s/internal/model"
)

func BearerAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		out, err := auth.VerifyToken(auth.VerifyTokenRequest{Token: token, Secret: secret})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("currentUser", &model.CurrentUser{
%s
		})
		c.Next()
	}
}
`, modulePath, assignBlock)
```

변경점:
- `"net/http"` import 추가
- 토큰 없음: `c.AbortWithStatusJSON(401, ...)` + return
- 토큰 검증 실패: `c.AbortWithStatusJSON(401, ...)` + return (기존 `c.Next()` → `c.Abort`)
- 주석 `// It does NOT abort` 삭제 (이제 abort 함)

### 2. `hurl_scenario.go` — 이메일 고유화

`renderFeatureHurl()` 또는 `buildScenarioBody()`에서 이메일 리터럴을 치환한다.

feature 파일명에서 prefix를 추출하여 이메일에 삽입:
- `course-lifecycle.feature` → prefix `lifecycle`
- `student-enrollment.feature` → prefix `enrollment`
- `course-deletion.feature` → prefix `deletion`

치환 규칙:
- `"inst@test.com"` → `"lifecycle-inst@test.com"` (feature별로 다름)
- `"student@test.com"` → `"enrollment-student@test.com"`

구현: `renderFeatureHurl()`에서 feature 파일명 기반 prefix를 계산하고, `writeActionHurlV2()`에 전달하여 JSON body 내 `@test.com` 패턴의 이메일 앞에 prefix를 삽입한다.

```go
func uniquifyEmails(json, prefix string) string {
    // Replace "xxx@test.com" with "prefix-xxx@test.com"
    re := regexp.MustCompile(`"([^"]+)@test\.com"`)
    return re.ReplaceAllString(json, fmt.Sprintf(`"%s-$1@test.com"`, prefix))
}
```

이 함수를 `buildScenarioBody()`가 body를 렌더링하기 전에 호출하여 JSON 원본의 이메일을 치환한다.

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `internal/gluegen/middlewaregen.go` | 수정 — 토큰 없으면 401 Abort, 검증 실패 시 401 Abort |
| `internal/gluegen/hurl_scenario.go` | 수정 — 이메일 리터럴에 feature명 prefix 삽입 |

## 의존성

없음 — gluegen 내부 수정.

## 검증 방법

```bash
go build ./cmd/fullend/
./fullend gen specs/dummy-lesson artifacts/dummy-lesson

# 1. 미들웨어 확인: abort 동작
grep -A5 "Bearer" artifacts/dummy-lesson/backend/internal/middleware/bearerauth.go

# 2. 이메일 고유화 확인
grep "@test.com" artifacts/dummy-lesson/tests/scenario-*.hurl artifacts/dummy-lesson/tests/invariant-*.hurl

# 3. 빌드 + 전체 hurl 테스트
cd artifacts/dummy-lesson/backend && go build -o server ./cmd/
# DB 초기화 + 서버 기동 + hurl --test (전체 통과)
```
