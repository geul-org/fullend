# ✅ 완료 — Phase 026: Func Request/Response 용어 전환 + Call 순서·타입 정합성 교차 검증

## 목표

1. **용어 전환**: Func Spec의 `*Input`/`*Output` → `*Request`/`*Response`로 통일 (gRPC, connect-go 관행 일치)
2. **Call 정합성 검증**: `@sequence call`에서 SSaC `@param`과 Func `Request` 필드의 **순서·타입** 정합성 검증. DDL(SymbolTable)과 OpenAPI에서 @param의 실제 타입을 추출하여 Request 필드 타입과 비교.

현재 `CheckFuncs()`는 함수 **존재 여부**와 **stub 여부**만 체크한다.

## Part 1: 용어 전환 (Input/Output → Request/Response)

### Func Spec 컨벤션 변경

Before:
```go
// @func issueToken
type IssueTokenInput struct { ... }
type IssueTokenOutput struct { ... }
func IssueToken(in IssueTokenInput) (IssueTokenOutput, error)
```

After:
```go
// @func issueToken
type IssueTokenRequest struct { ... }
type IssueTokenResponse struct { ... }
func IssueToken(req IssueTokenRequest) (IssueTokenResponse, error)
```

### 변경 파일

| 파일 | 변경 |
|---|---|
| `pkg/auth/hash_password.go` | `HashPasswordInput/Output` → `Request/Response`, `in` → `req` |
| `pkg/auth/verify_password.go` | `VerifyPasswordInput/Output` → `Request/Response` |
| `pkg/auth/issue_token.go` | `IssueTokenInput/Output` → `Request/Response` |
| `pkg/auth/verify_token.go` | `VerifyTokenInput/Output` → `Request/Response` |
| `pkg/auth/refresh_token.go` | `RefreshTokenInput/Output` → `Request/Response` |
| `pkg/auth/generate_reset_token.go` | `GenerateResetTokenInput/Output` → `Request/Response` |
| `pkg/crypto/encrypt.go` | `EncryptInput/Output` → `Request/Response` |
| `pkg/crypto/decrypt.go` | `DecryptInput/Output` → `Request/Response` |
| `pkg/crypto/generate_otp.go` | `GenerateOTPInput/Output` → `Request/Response` |
| `pkg/crypto/verify_otp.go` | `VerifyOTPInput/Output` → `Request/Response` |
| `pkg/storage/upload_file.go` | `UploadFileInput/Output` → `Request/Response` |
| `pkg/storage/delete_file.go` | `DeleteFileInput/Output` → `Request/Response` |
| `pkg/storage/presign_url.go` | `PresignURLInput/Output` → `Request/Response` |
| `pkg/mail/send_email.go` | `SendEmailInput/Output` → `Request/Response` |
| `pkg/mail/send_template_email.go` | `SendTemplateEmailInput/Output` → `Request/Response` |
| `pkg/text/generate_slug.go` | `GenerateSlugInput/Output` → `Request/Response` |
| `pkg/text/sanitize_html.go` | `SanitizeHTMLInput/Output` → `Request/Response` |
| `pkg/text/truncate_text.go` | `TruncateTextInput/Output` → `Request/Response` |
| `pkg/image/og_image.go` | `OgImageInput/Output` → `Request/Response` |
| `pkg/image/thumbnail.go` | `ThumbnailInput/Output` → `Request/Response` |
| `artifacts/internal/funcspec/parser.go` | `expectedInput/Output` → `*Request/*Response`, `InputFields/OutputFields` → `RequestFields/ResponseFields` |
| `artifacts/internal/funcspec/parser_test.go` | 기대값 갱신 |
| `artifacts/internal/crosscheck/func.go` | skeleton 템플릿의 `Input/Output` → `Request/Response` |
| `artifacts/manual-for-ai.md` | Func Spec 규칙·예시 갱신 |
| `README.md` | 변경 없음 (Input/Output 언급 없음) |

## Part 2: Call 순서·타입 정합성 교차 검증

### 타입 정보 출처

SSaC `@param`에는 타입 정보가 없다. 하지만 Source를 추적하면 DDL/OpenAPI에서 타입을 알 수 있다.

| @param 형태 | Source | 타입 출처 |
|---|---|---|
| `@param user.PasswordHash` | 변수.필드 | SymbolTable → User 테이블 → PasswordHash 컬럼 → Go 타입 |
| `@param Password request` | request | OpenAPIDoc → operationId → request schema → Password → 타입 |
| `@param "리터럴"` | 리터럴 | string 고정 |

**Go 타입 매핑**: DDL 컬럼 타입과 OpenAPI 타입을 Go 타입으로 변환하여 Request 필드 타입과 비교.

| DDL 타입 | Go 타입 |
|---|---|
| `TEXT`, `VARCHAR` | `string` |
| `INTEGER`, `BIGINT`, `SERIAL`, `BIGSERIAL` | `int64` |
| `BOOLEAN` | `bool` |
| `TIMESTAMP`, `TIMESTAMPTZ` | `time.Time` |
| `BYTEA` | `[]byte` |

| OpenAPI 타입 | Go 타입 |
|---|---|
| `string` | `string` |
| `integer` (int64) | `int64` |
| `integer` (int32) | `int32` |
| `boolean` | `bool` |
| `number` | `float64` |
| `string` (format: date-time) | `time.Time` |

### 규칙 1: 개수 일치 (ERROR)

```
@sequence call
@func auth.verifyPassword
@param user.PasswordHash        ← 2개 (리터럴 제외하지 않음)
@param Password request
```

`VerifyPasswordRequest` 필드 2개. **@param 개수 = RequestFields 개수**여야 한다.

- 불일치 → **ERROR**
- 메시지: `@func auth.verifyPassword — @param 2개, Request 필드 3개 (불일치)`

### 규칙 2: 순서별 타입 일치 (ERROR)

```
@param user.PasswordHash       ← 1번째, DDL User.PasswordHash → string
@param Password request        ← 2번째, OpenAPI Password → string

Request 필드:
    PasswordHash string        ← 1번째, string ✓ 일치
    Password     string        ← 2번째, string ✓ 일치
```

i번째 @param의 추출 타입과 i번째 Request 필드의 Go 타입을 비교.

- 불일치 → **ERROR**
- 메시지: `@func auth.issueToken — 1번째 param(int64) ≠ 1번째 Request 필드 ID(string) 타입 불일치`
- 타입 추출 실패 시 → **스킵** (타입 정보가 없으면 비교하지 않음)

### 규칙 3: Result ↔ Response 유무 일치 (ERROR/WARNING)

```
@func auth.issueToken
@result token Token
```

- **@result 있는데 Response 필드 0개** → ERROR
- **@result 없는데 Response 필드 1개 이상** → WARNING (반환값 무시)

### 규칙 4: Param source 변수 선행 정의 (WARNING)

```
@sequence get
@model User.FindByEmail
@result user User              ← user 정의

@sequence call
@func auth.verifyPassword
@param user.PasswordHash       ← user 참조
```

`@param`의 Source(`user`)가 선행 `@result`의 Var에 정의되어 있는지 확인.

- `request` → 항상 유효 (HTTP request body)
- 리터럴(`"..."`) → 항상 유효
- 그 외 변수명 → 선행 @result.Var에 존재해야 함
- 미정의 → **WARNING**

### CheckFuncs 시그니처 변경

```go
// Before
func CheckFuncs(serviceFuncs []ssacparser.ServiceFunc, fullendPkgSpecs, projectFuncSpecs []funcspec.FuncSpec) []CrossError

// After
func CheckFuncs(
    serviceFuncs    []ssacparser.ServiceFunc,
    fullendPkgSpecs []funcspec.FuncSpec,
    projectFuncSpecs []funcspec.FuncSpec,
    symbolTable     *ssacvalidator.SymbolTable,
    openAPIDoc      *openapi3.T,
) []CrossError
```

### 변경 파일

| 파일 | 변경 |
|---|---|
| `artifacts/internal/crosscheck/func.go` | 규칙 1~4 + 타입 추출 로직, 시그니처에 SymbolTable/OpenAPIDoc 추가 |
| `artifacts/internal/crosscheck/func_test.go` | 신규 — 4개 규칙 테스트 |
| `artifacts/internal/crosscheck/crosscheck.go` | `CheckFuncs()` 호출부에 SymbolTable/OpenAPIDoc 전달 |

## 의존성

- `funcspec.FuncSpec.RequestFields` / `ResponseFields` — Part 1에서 리네임
- `ssacparser.Sequence.Params` (Name, Source) — 이미 파싱됨
- `ssacparser.Sequence.Result` (Var, Type) — 이미 파싱됨
- `ssacvalidator.SymbolTable` — 이미 CrossValidateInput에 존재
- `openapi3.T` — 이미 CrossValidateInput에 존재
- 추가 외부 의존 없음

## 검증 방법

1. `go build ./...` — 빌드 통과
2. `go test ./artifacts/internal/funcspec/...` — 파서 테스트 통과
3. `go test ./artifacts/internal/crosscheck/...` — 교차 검증 테스트 통과
4. `go run ./artifacts/cmd/fullend validate specs/dummy-lesson/` — 에러 없이 통과
