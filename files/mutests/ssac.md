# Mutation Test — SSaC 시퀀스 타입별 문법 오류 검출력 검증

## 목적

SSaC 시퀀스 타입별로 입력 하나씩 교묘하게 틀리게 하여 `fullend validate`가 검출하는지 확인한다.
검증 레이어: SSaC Parser → SSaC Validator → Crosscheck (순서대로 잡힘).

## 방법

1. 대상 `.ssac` 파일에 변경 적용
2. `go run ./cmd/fullend validate specs/gigbridge` 실행
3. 검출 여부 기록 (PASS = 잡음, FAIL = 못 잡음)
4. `git checkout -- specs/gigbridge/` 로 되돌림

---

## @get (6건)

### MUT-SSAC-001: Model 누락
- 대상: `service/gig/get_gig.ssac`
- 변경: `@get Gig gig = Gig.FindByID({ID: request.id})` → `@get Gig gig = ({ID: request.id})`
- 기대: ERROR — Validator: Model required for @get

### MUT-SSAC-002: Result 타입 오타
- 대상: `service/gig/get_gig.ssac`
- 변경: `@get Gig gig = Gig.FindByID(...)` → `@get Gigs gig = Gig.FindByID(...)`
- 기대: ERROR — Crosscheck: @result type "Gigs" has no matching DDL table

### MUT-SSAC-003: Result 변수명 대소문자
- 대상: `service/gig/get_gig.ssac`
- 변경: `@get Gig gig = ...` → `@get Gig Gig = ...` 후 `@empty gig` 그대로
- 기대: ERROR — Validator: variable "gig" not declared (Gig으로 선언됨)

### MUT-SSAC-004: Model.Method 에서 dot 누락
- 대상: `service/gig/get_gig.ssac`
- 변경: `Gig.FindByID` → `GigFindByID`
- 기대: ERROR — Validator: Model format must be "Model.Method"

### MUT-SSAC-005: Input key 오타
- 대상: `service/gig/get_gig.ssac`
- 변경: `{ID: request.id}` → `{Id: request.id}`
- 기대: WARNING — Crosscheck: SSaC input key "Id" not in DDL columns (검출 안 될 수 있음 — sqlc 메서드 파라미터와 대조)

### MUT-SSAC-006: Input source 미선언 변수 참조
- 대상: `service/gig/get_gig.ssac`
- 변경: `{ID: request.id}` → `{ID: unknown.id}`
- 기대: ERROR — Validator: source "unknown" not declared

---

## @post (5건)

### MUT-SSAC-007: Inputs 누락 (@post은 필수)
- 대상: `service/gig/create_gig.ssac`
- 변경: `@post Gig gig = Gig.Create({...})` → `@post Gig gig = Gig.Create()`
- 기대: ERROR — Validator: @post requires inputs

### MUT-SSAC-008: Result 누락 (@post은 필수)
- 대상: `service/gig/create_gig.ssac`
- 변경: `@post Gig gig = Gig.Create({...})` → `@post Gig.Create({...})`
- 기대: ERROR — Validator: @post requires result assignment

### MUT-SSAC-009: request 필드 대소문자 (OpenAPI snake_case vs camelCase)
- 대상: `service/gig/create_gig.ssac`
- 변경: `request.title` → `request.Title`
- 기대: ERROR — Crosscheck: SSaC @post field "Title" not in OpenAPI CreateGig request schema (OpenAPI는 snake_case "title")

### MUT-SSAC-010: currentUser 오타
- 대상: `service/gig/create_gig.ssac`
- 변경: `currentUser.ID` → `CurrentUser.ID`
- 기대: ERROR — Validator: source "CurrentUser" not declared

### MUT-SSAC-011: 문자열 리터럴 따옴표 누락
- 대상: `service/gig/publish_gig.ssac`
- 변경: `{Status: "open", ID: gig.ID}` → `{Status: open, ID: gig.ID}`
- 기대: ERROR — Validator: source "open" not declared (변수로 해석됨)

---

## @put (4건)

### MUT-SSAC-012: Result 할당 시도 (@put은 금지)
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@put Gig.UpdateStatus({...})` → `@put Gig result = Gig.UpdateStatus({...})`
- 기대: ERROR — Validator: @put must not have result

### MUT-SSAC-013: Inputs 누락 (@put은 필수)
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@put Gig.UpdateStatus({Status: "open", ID: gig.ID})` → `@put Gig.UpdateStatus()`
- 기대: ERROR — Validator: @put requires inputs

### MUT-SSAC-014: Model 대소문자 (Model vs model)
- 대상: `service/gig/publish_gig.ssac`
- 변경: `Gig.UpdateStatus` → `gig.UpdateStatus`
- 기대: ERROR — Crosscheck: SSaC @result type "gig" has no matching DDL table (or 다른 에러)

### MUT-SSAC-015: 선언 안 된 변수 사용
- 대상: `service/gig/publish_gig.ssac`
- 변경: `{Status: "open", ID: gig.ID}` → `{Status: "open", ID: order.ID}`
- 기대: ERROR — Validator: source "order" not declared

---

## @delete (2건)

### MUT-SSAC-016: Result 할당 시도 (@delete은 금지)
- 대상: 없음 — gigbridge에 @delete 없음. 인라인 테스트로 대체.
- 변경: 임의 ssac에 `@delete Gig result = Gig.Delete({ID: gig.ID})` 추가
- 기대: ERROR — Validator: @delete must not have result

### MUT-SSAC-017: Inputs 없는 @delete (WARNING)
- 대상: 없음 — 인라인 테스트로 대체.
- 변경: `@delete Gig.Delete()` 추가 (suppress 없이)
- 기대: WARNING — Validator: @delete with no inputs

---

## @empty (4건)

### MUT-SSAC-018: Target 미선언 변수
- 대상: `service/gig/get_gig.ssac`
- 변경: `@empty gig "Gig not found"` → `@empty order "Gig not found"`
- 기대: ERROR — Validator: target variable "order" not declared

### MUT-SSAC-019: Message 누락
- 대상: `service/gig/get_gig.ssac`
- 변경: `@empty gig "Gig not found"` → `@empty gig`
- 기대: ERROR — Validator: @empty requires message

### MUT-SSAC-020: ErrStatus 범위 밖
- 대상: `service/gig/get_gig.ssac`
- 변경: `@empty gig "Gig not found"` → `@empty gig "Gig not found" 999`
- 기대: ERROR or WARNING — status code 999는 유효하지 않음 (검출 안 될 수 있음 — 파서가 100-599만 허용하는지 확인 필요)

### MUT-SSAC-021: @empty→OpenAPI 404 응답 대조
- 대상: `service/gig/get_gig.ssac`
- 변경: `@empty gig "Gig not found"` → `@empty gig "Gig not found" 400`
- 기대: ERROR — Crosscheck: SSaC @empty uses HTTP 400 but OpenAPI GetGig has no 400 response (MUT-18과 역방향)

---

## @exists (2건)

### MUT-SSAC-022: Target 미선언 변수
- 대상: gigbridge에 @exists 없음 — 인라인 테스트로 대체.
- 변경: 임의 ssac에 `@exists unknown "Already exists"` 추가
- 기대: ERROR — Validator: target variable "unknown" not declared

### MUT-SSAC-023: @exists 기본 409와 OpenAPI 응답 대조
- 대상: 인라인 테스트로 대체.
- 변경: `@exists user "Already exists"` (OpenAPI에 409 응답 없음)
- 기대: ERROR — Crosscheck: SSaC @exists uses HTTP 409 but OpenAPI has no 409 response

---

## @state (5건)

### MUT-SSAC-024: DiagramID 오타
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@state gig {status: gig.Status} "PublishGig"` → `@state gigs {status: gig.Status} "PublishGig"`
- 기대: ERROR — Crosscheck: diagram "gigs" not found

### MUT-SSAC-025: Transition 이벤트명 오타
- 대상: `service/gig/publish_gig.ssac`
- 변경: `"PublishGig"` → `"publishGig"` (transition 문자열)
- 기대: ERROR — Crosscheck: "publishGig" is not a valid transition event in diagram "gig"

### MUT-SSAC-026: Inputs 누락 (@state은 필수)
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@state gig {status: gig.Status} "PublishGig" "Cannot transition"` → `@state gig "PublishGig" "Cannot transition"`
- 기대: ERROR — Validator: @state requires inputs

### MUT-SSAC-027: Input 변수 참조 오타 (Status 필드)
- 대상: `service/gig/publish_gig.ssac`
- 변경: `{status: gig.Status}` → `{status: gig.State}`
- 기대: WARNING or 무검출 — DDL 칼럼 "state" 부재 여부 (crosscheck 범위 외일 수 있음)

### MUT-SSAC-028: Message 누락
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@state gig {status: gig.Status} "PublishGig" "Cannot transition"` → `@state gig {status: gig.Status} "PublishGig"`
- 기대: ERROR — Validator: @state requires message

---

## @auth (5건)

### MUT-SSAC-029: Action 대소문자
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@auth "PublishGig" "gig"` → `@auth "publishGig" "gig"`
- 기대: ERROR — Crosscheck: SSaC authorize (publishGig, gig) has no matching Rego rule

### MUT-SSAC-030: Resource 오타
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@auth "PublishGig" "gig"` → `@auth "PublishGig" "gigs"`
- 기대: ERROR — Crosscheck: SSaC authorize (PublishGig, gigs) has no matching Rego rule

### MUT-SSAC-031: Action 따옴표 누락
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@auth "PublishGig" "gig"` → `@auth PublishGig "gig"`
- 기대: ERROR — Parser: action must be quoted string

### MUT-SSAC-032: Message 누락
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@auth "PublishGig" "gig" {UserID: currentUser.ID, ResourceID: gig.ID} "Forbidden"` → `@auth "PublishGig" "gig" {UserID: currentUser.ID, ResourceID: gig.ID}`
- 기대: ERROR — Validator: @auth requires message

### MUT-SSAC-033: Input 변수 오타
- 대상: `service/gig/publish_gig.ssac`
- 변경: `{UserID: currentUser.ID, ResourceID: gig.ID}` → `{UserID: currentUser.ID, ResourceID: order.ID}`
- 기대: ERROR — Validator: source "order" not declared

---

## @call (5건)

### MUT-SSAC-034: 패키지명 대소문자
- 대상: `service/auth/login.ssac`
- 변경: `auth.IssueToken` → `Auth.IssueToken`
- 기대: ERROR — Crosscheck: @call Auth.issueToken — 구현 없음

### MUT-SSAC-035: 함수명 오타
- 대상: `service/auth/login.ssac`
- 변경: `auth.IssueToken` → `auth.issueToken`
- 기대: ERROR — Crosscheck: func "issueToken" not found in package "auth"

### MUT-SSAC-036: Result 타입에 primitive 사용 (@call은 금지)
- 대상: `service/auth/login.ssac`
- 변경: `@call auth.IssueTokenResponse token = auth.IssueToken(...)` → `@call string token = auth.IssueToken(...)`
- 기대: ERROR — Validator: @call result type cannot be primitive

### MUT-SSAC-037: Input 타입 불일치
- 대상: `service/proposal/accept_proposal.ssac`
- 변경: `billing.HoldEscrow({GigID: gig.ID, ...})` → `billing.HoldEscrow({GigID: gig.Budget, ...})` (int64 vs int64 — 동일, 변경 의미 없음)
- 변경 (대안): Input key 오타 `{GigId: gig.ID, ...}` → FuncSpec의 `GigID` 와 불일치
- 기대: ERROR — Crosscheck: @call Input 필드 "GigId"가 HoldEscrowRequest에 없음

### MUT-SSAC-038: Model 없이 @call
- 대상: `service/auth/login.ssac`
- 변경: `@call auth.VerifyPassword({...})` → `@call ({...})`
- 기대: ERROR — Validator: @call requires Model

---

## @publish (3건 — gigbridge에 @publish 없음, 문법 기반 인라인)

### MUT-SSAC-039: Topic 따옴표 누락
- 대상: 인라인
- 변경: `@publish "order.completed" {OrderID: order.ID}` → `@publish order.completed {OrderID: order.ID}`
- 기대: ERROR — Parser: topic must be quoted string

### MUT-SSAC-040: Payload 누락 (@publish은 필수)
- 대상: 인라인
- 변경: `@publish "order.completed" {OrderID: order.ID}` → `@publish "order.completed"`
- 기대: ERROR — Validator: @publish requires inputs

### MUT-SSAC-041: Topic ↔ @subscribe 불일치
- 대상: 인라인
- 변경: `@publish "order.completed"` → `@publish "order.complete"` (오타)
- 기대: WARNING — Crosscheck: topic "order.complete" has no matching @subscribe

---

## @response (5건)

### MUT-SSAC-042: 필드명 오타 (OpenAPI 응답 property 불일치)
- 대상: `service/gig/create_gig.ssac`
- 변경: `@response { gig: gig }` → `@response { Gig: gig }`
- 기대: ERROR — Crosscheck: @response field "Gig" not in OpenAPI CreateGig response schema

### MUT-SSAC-043: 변수 참조 오타
- 대상: `service/gig/create_gig.ssac`
- 변경: `@response { gig: gig }` → `@response { gig: order }`
- 기대: ERROR — Validator: response variable "order" not declared

### MUT-SSAC-044: Shorthand 변수 오타
- 대상: `service/gig/list_gigs.ssac`
- 변경: `@response gigPage` → `@response gigPages`
- 기대: ERROR — Validator: response variable "gigPages" not declared

### MUT-SSAC-045: @response 뒤 닫는 중괄호 누락
- 대상: `service/gig/create_gig.ssac`
- 변경: `@response { gig: gig }` → `@response { gig: gig`
- 기대: ERROR — Parser: unclosed @response block

### MUT-SSAC-046: dotted field 참조
- 대상: `service/auth/register.ssac`
- 변경: `@response { user: user }` → `@response { user: hp.HashedPassword }`
- 기대: PASS or WARNING — 필드 타입 불일치 (string vs User) — crosscheck 범위 외일 수 있음

---

## 변수 흐름 (3건)

### MUT-SSAC-047: 선언 전 사용
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@empty gig`와 `@get Gig gig = ...` 순서 교체 — `@empty` 먼저
- 기대: ERROR — Validator: target variable "gig" not declared

### MUT-SSAC-048: @put 이후 stale 변수 사용
- 대상: `service/gig/publish_gig.ssac`
- 변경: `@put` 이후 re-@get 삭제 → `@response { gig: gig }` (stale)
- 기대: WARNING — Validator: variable "gig" may be stale after @put

### MUT-SSAC-049: @call 결과 변수 재사용 (동일 이름 재선언)
- 대상: `service/auth/login.ssac`
- 변경: `@call auth.IssueTokenResponse token = ...` → `@call auth.IssueTokenResponse user = ...` 후 `@response user`
- 기대: PASS — 기존 user(@get 결과)를 @call 결과로 재할당. codegen에서 `:=` → `=` 전환이 정상 동작하는지

---

## 결과 기록

| ID | 타입 | 변경 | 기대 | 실제 | 비고 |
|---|---|---|---|---|---|
| MUT-SSAC-001 | @get | Model 누락 | PASS | | |
| MUT-SSAC-002 | @get | Result 타입 오타 Gigs | PASS | | |
| MUT-SSAC-003 | @get | 변수명 대소문자 Gig/gig | PASS | | |
| MUT-SSAC-004 | @get | dot 누락 GigFindByID | PASS | | |
| MUT-SSAC-005 | @get | Input key Id→ID | ? | | |
| MUT-SSAC-006 | @get | 미선언 변수 unknown.id | PASS | | |
| MUT-SSAC-007 | @post | Inputs 누락 | PASS | | |
| MUT-SSAC-008 | @post | Result 누락 | PASS | | |
| MUT-SSAC-009 | @post | request.Title (대소문자) | PASS | | |
| MUT-SSAC-010 | @post | CurrentUser 오타 | PASS | | |
| MUT-SSAC-011 | @post | 따옴표 누락 open | PASS | | |
| MUT-SSAC-012 | @put | Result 할당 시도 | PASS | | |
| MUT-SSAC-013 | @put | Inputs 누락 | PASS | | |
| MUT-SSAC-014 | @put | Model 소문자 gig.Update | PASS | | |
| MUT-SSAC-015 | @put | 미선언 변수 order.ID | PASS | | |
| MUT-SSAC-016 | @delete | Result 할당 시도 | PASS | | |
| MUT-SSAC-017 | @delete | Inputs 없음 | PASS | | |
| MUT-SSAC-018 | @empty | 미선언 변수 order | PASS | | |
| MUT-SSAC-019 | @empty | Message 누락 | PASS | | |
| MUT-SSAC-020 | @empty | ErrStatus 999 | ? | | |
| MUT-SSAC-021 | @empty | ErrStatus 400 vs OpenAPI | PASS | | |
| MUT-SSAC-022 | @exists | 미선언 변수 | PASS | | |
| MUT-SSAC-023 | @exists | 409 vs OpenAPI | PASS | | |
| MUT-SSAC-024 | @state | DiagramID 오타 gigs | PASS | | |
| MUT-SSAC-025 | @state | Transition 대소문자 | PASS | | |
| MUT-SSAC-026 | @state | Inputs 누락 | PASS | | |
| MUT-SSAC-027 | @state | 필드 오타 State→Status | ? | | |
| MUT-SSAC-028 | @state | Message 누락 | PASS | | |
| MUT-SSAC-029 | @auth | Action 대소문자 | PASS | | |
| MUT-SSAC-030 | @auth | Resource 오타 gigs | PASS | | |
| MUT-SSAC-031 | @auth | Action 따옴표 누락 | PASS | | |
| MUT-SSAC-032 | @auth | Message 누락 | PASS | | |
| MUT-SSAC-033 | @auth | 미선언 변수 order.ID | PASS | | |
| MUT-SSAC-034 | @call | 패키지명 대소문자 Auth | PASS | | |
| MUT-SSAC-035 | @call | 함수명 소문자 issueToken | PASS | | |
| MUT-SSAC-036 | @call | Result 타입 primitive | PASS | | |
| MUT-SSAC-037 | @call | Input key 오타 GigId | PASS | | |
| MUT-SSAC-038 | @call | Model 누락 | PASS | | |
| MUT-SSAC-039 | @publish | Topic 따옴표 누락 | PASS | | |
| MUT-SSAC-040 | @publish | Payload 누락 | PASS | | |
| MUT-SSAC-041 | @publish | Topic 오타 | PASS | | |
| MUT-SSAC-042 | @response | 필드명 대소문자 Gig | PASS | | |
| MUT-SSAC-043 | @response | 변수 참조 오타 order | PASS | | |
| MUT-SSAC-044 | @response | Shorthand 오타 gigPages | PASS | | |
| MUT-SSAC-045 | @response | 중괄호 누락 | PASS | | |
| MUT-SSAC-046 | @response | dotted field 타입 | ? | | |
| MUT-SSAC-047 | 변수흐름 | 선언 전 사용 | PASS | | |
| MUT-SSAC-048 | 변수흐름 | stale 변수 사용 | PASS | | |
| MUT-SSAC-049 | 변수흐름 | 변수 재선언 | PASS | | |

**`?` 표시**: 검출 여부가 불확실한 항목 (현재 crosscheck/validator 범위 밖일 수 있음)
