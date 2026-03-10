# Custom Func — fullend의 마지막 퍼즐

## 배경

fullend SSOT 체계에서 코드젠으로 커버되지 않는 유일한 영역: **외부 함수**.
SSaC `@sequence call`이 참조하는 함수들 (hashPassword, verifyPassword, issueToken 등)은
정형화된 패턴이 아니라 프로젝트마다 구현이 다르다.

비밀번호 검증도 기존 `@sequence password`(bcrypt 하드코딩)에서 `@sequence call`로 통합.
시퀀스 타입 11→10. 비정형 로직은 전부 call로 일원화.

## 핵심 아이디어

1. **스펙은 선언**: Input/Output struct + `@description` 자연어 한 줄
2. **구현은 LLM**: 스켈레톤만 코드젠하고, 본체는 LLM이 `@description`을 보고 작성
3. **패키지 함수**: `@func auth.hashPassword` → `auth.HashPassword()` 패키지 레벨 함수 호출
4. **규격화된 계약**: 항상 `func(In) (Out, error)` — 동일 계약의 구현체를 교체 가능

## 책임 분리: ssac vs fullend

| 책임 | ssac | fullend |
|---|---|---|
| `@func auth.hashPassword` 파싱 | ✓ (Package, Func 필드) | |
| 서비스 코드에서 호출 코드 생성 | ✓ (`auth.HashPassword(auth.HashPasswordInput{...})`) | |
| import 처리 | ✓ (SSaC 스펙 파일의 import를 그대로 생성 코드에 옮김) | |
| Input/Output struct 정의 | | ✓ (func 스펙 파일) |
| func 스켈레톤 생성 | | ✓ (gen 단계) |
| func 본체 구현 | | ✓ → LLM 위임 |
| Func ↔ SSaC 교차 검증 | | ✓ |
| `pkg/` 기본 구현 관리 | | ✓ |

## SSaC 호출 측

```go
package service

import (
    "net/http"
    auth "github.com/geul-org/fullend/pkg/auth"
)

// @sequence call
// @func auth.hashPassword
// @param Password request
// @result HashedPassword string

// guard형 — @result 없음, @message로 실패 응답 선언
// @sequence call
// @func auth.verifyPassword
// @param user.PasswordHash
// @param Password request
// @message "비밀번호가 일치하지 않습니다" 401
```

### SSaC 생성 코드

```go
// value형 (@result 있음)
out, err := auth.HashPassword(auth.HashPasswordInput{Password: password})
if err != nil {
    http.Error(w, "hashPassword 호출 실패", http.StatusInternalServerError)
    return
}
hashedPassword := out.HashedPassword

// guard형 (@result 없음, @message 있음)
_, err = auth.VerifyPassword(auth.VerifyPasswordInput{
    PasswordHash: user.PasswordHash,
    Password:     password,
})
if err != nil {
    http.Error(w, "비밀번호가 일치하지 않습니다", http.StatusUnauthorized)
    return
}
```

### `@message` 규칙

- `@message "메시지" STATUS` — 실패 시 HTTP 응답 (메시지 + 상태 코드)
- `@message` 없으면 기본값: `"funcName 호출 실패" 500`
- 구현 측은 `error`만 반환. 실패 사유와 HTTP status는 SSaC 선언에서 결정

## 시그니처 규칙

- 입력: 항상 **단일 struct** `FuncNameInput`
- 출력: 항상 **단일 struct** `FuncNameOutput` + `error`
- `@result` 없는 guard형도 빈 Output struct (시그니처 통일)
- 시그니처 통일의 이유:
  - 코드젠 단순화 — 분기 없이 항상 `func(In) (Out, error)`
  - 동일 계약의 구현체 교체 가능
  - 나중에 output 필드가 추가되어도 시그니처 변경 없음

## Func 스펙 파일 구조

### 프로젝트별 (specs 내)
```
specs/dummy-lesson/func/
└── auth/
    ├── hash_password.go      # 프로젝트 커스텀 구현
    ├── verify_password.go
    └── issue_token.go
```

### fullend 기본 제공 (fullend 루트)
```
pkg/
├── auth/
│   ├── hash_password.go      # bcrypt 기본 구현
│   ├── verify_password.go    # bcrypt 기본 구현
│   └── issue_token.go        # JWT 기본 구현
├── payment/
│   └── process_payment.go    # 스텁 구현
└── notification/
    └── send_email.go         # 스텁 구현
```

### Fallback 체인

1. `specs/<project>/func/auth/hash_password.go` ← 프로젝트 커스텀 (최우선)
2. `pkg/auth/hash_password.go` ← fullend 기본 제공 (fallback)
3. 둘 다 없으면 → 스켈레톤 생성 (`// TODO: implement`) + WARNING

## Func 파일 형식

```go
package auth

import "golang.org/x/crypto/bcrypt"

// @func hashPassword
// @description 평문 비밀번호를 안전한 해시로 변환한다

type HashPasswordInput struct {
    Password string
}

type HashPasswordOutput struct {
    HashedPassword string
}

func HashPassword(in HashPasswordInput) (HashPasswordOutput, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
    return HashPasswordOutput{HashedPassword: string(hash)}, err
}
```

핵심:
- **`@func`**: 함수 식별자 (SSaC `@func auth.hashPassword`의 `hashPassword`와 매칭)
- **`@description`**: 자연어 한 줄. LLM이 이것만 보고 본체를 구현
- **Input/Output struct**: Go struct가 곧 스펙. 별도 어노테이션 불필요
- **패키지 레벨 함수**: `func HashPassword(in) (out, error)` — Service struct 불필요

## fullend gen 흐름

1. SSaC 스펙의 import에서 func 패키지 경로 확인
2. 로컬 패키지인 경우:
   → `specs/<project>/func/auth/` → `artifacts/<project>/backend/internal/auth/`로 복사
3. 외부 모듈인 경우 (`github.com/geul-org/fullend/pkg/auth`):
   → 복사 불필요. `go get`으로 의존성 해결

## fullend validate 흐름

1. SSaC에서 `@func` 참조 + import 경로 수집
2. import가 `github.com/geul-org/fullend/pkg/` → fullend 기본 제공, 검증 스킵
3. import가 로컬 경로 → `specs/<project>/func/<pkg>/`에 구현 파일 존재 확인
   - 없으면 **ERROR** + 구현 지침 프롬프트 출력:
     ```
     ERROR: @func billing.calculateRefund — 구현 없음

     다음 파일을 작성하세요: specs/<project>/func/billing/calculate_refund.go

     package billing

     // @func calculateRefund
     // @description <이 함수가 무엇을 하는지 한 줄로 설명>

     type CalculateRefundInput struct {
         Reservation Reservation
     }

     type CalculateRefundOutput struct {
         Refund Refund
     }

     func CalculateRefund(in CalculateRefundInput) (CalculateRefundOutput, error) {
         // TODO: implement
         return CalculateRefundOutput{}, nil
     }
     ```
     Input/Output struct는 SSaC `@param`/`@result`에서 자동 유도.
     `@description`만 채우고 본체를 구현하면 됨. LLM에 이 에러 그대로 전달 가능.
4. 파일이 있으면:
   - Input struct 필드 ↔ SSaC `@param` 교차 검증 (ERROR)
   - Output struct 필드 ↔ SSaC `@result` 교차 검증 (ERROR)
   - 본체가 `// TODO: implement`이면 WARNING

## SSOT 완성도

| 계층 | 선언 (SSOT) | 구현 |
|---|---|---|
| API | OpenAPI | 코드젠 |
| DB | DDL | 코드젠 (sqlc) |
| 서비스 흐름 | SSaC | 코드젠 |
| 외부 함수 | **Func spec** (struct + @description) | **LLM** (fallback: fullend pkg 기본) |
| UI | STML | 코드젠 |
| 상태 전이 | States | 코드젠 |
| 인가 정책 | Policy | 코드젠 |
| 시나리오 테스트 | Scenario | 코드젠 |
| 인프라 | Terraform | fmt만 |

외부 함수만 유일하게 LLM이 본체를 채우는 계층.
정형화 가능한 건 코드젠, 불가능한 건 LLM.

## 결론

- [x] Func spec은 SSaC 확장이 아니라 **fullend가 관리하는 별도 파일**
- [x] 패키지 레벨 함수 (Service struct 불필요)
- [x] import 경로는 SSaC 스펙 파일에서 직접 선언
- [ ] Input/Output struct의 타입이 model 참조할 때 (예: `User`, `Token`) import 규칙?
