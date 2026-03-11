# fullend — AI SSOT Integration Guide

> SSaC, STML, Func Spec, Mermaid stateDiagram, OPA Rego, Gherkin, OpenAPI x- 확장, 교차 검증 규칙, pkg/ 함수·모델 전체 문법 참조.
> OpenAPI/SQL DDL/Terraform 문법은 설명하지 않음.

## Project Directory Structure

```
<project-root>/
├── fullend.yaml                  # Project config (required)
├── api/openapi.yaml              # OpenAPI 3.x (with x- extensions)
├── db/
│   ├── *.sql                     # DDL (CREATE TABLE, CREATE INDEX)
│   └── queries/*.sql             # sqlc queries (-- name: Method :cardinality)
├── service/**/*.ssac             # SSaC declarations (.ssac extension, Go comment DSL)
├── model/*.go                    # Go structs (// @dto for non-DDL types)
├── func/<pkg>/*.go               # Custom func implementations (optional)
├── states/*.md                   # Mermaid stateDiagram (state transitions)
├── policy/*.rego                 # OPA Rego (authorization policies)
├── scenario/*.feature            # Gherkin scenarios (fixed-pattern)
├── frontend/
│   ├── *.html                    # STML declarations (HTML5 + data-*)
│   ├── *.custom.ts               # Frontend computed functions (optional)
│   └── components/*.tsx          # React component wrappers (optional)
└── terraform/*.tf                # HCL infrastructure declarations
```

## fullend.yaml

```yaml
apiVersion: fullend/v1
kind: Project

metadata:
  name: <project-name>

backend:
  lang: go
  framework: gin
  module: github.com/org/project
  middleware:
    - bearerAuth                    # Must match OpenAPI securitySchemes keys
  auth:
    secret_env: JWT_SECRET
    claims:                         # JWT claims → CurrentUser field mapping
      ID: user_id                   # *_id → int64, otherwise → string
      Email: email
      Role: role

frontend:
  lang: typescript
  framework: react
  bundler: vite
  name: project-web

deploy:
  image: ghcr.io/org/project
  domain: project.example.com

session:
  backend: postgres                 # postgres | memory

cache:
  backend: postgres                 # postgres | memory

file:
  backend: s3                       # s3 | local
  s3:
    bucket: my-bucket
    region: ap-northeast-2
  local:
    root: ./uploads
```

### Required Fields

`apiVersion` (fullend/v1), `kind` (Project), `metadata.name`, `backend.module`

### Optional Fields

| Field | Description |
|---|---|
| `backend.auth.claims` | JWT claims → `CurrentUser` struct 생성 |
| `session.backend` | Session backend: `postgres` or `memory` |
| `cache.backend` | Cache backend: `postgres` or `memory` |
| `file.backend` | File storage: `s3` or `local` |

## SSaC — Service Logic Declarations

### File Extension: `.ssac`

Go 문법 그대로 사용하되, `.ssac` 확장자로 Go 빌드 대상에서 제외.

```go
package service

import "github.com/geul-org/fullend/pkg/auth"

// @call auth.HashPasswordResponse hp = auth.HashPassword({Password: request.Password})
// @post User user = User.Create({Email: request.Email, PasswordHash: hp.HashedPassword})
// @response { user: user }
func Register() {}
```

### 10 Sequence Types

| Type | Purpose | Format | Args |
|---|---|---|---|
| `@get` | Query | `Type var = Model.Method(args...)` | 0개 허용 |
| `@post` | Create | `Type var = Model.Method(args...)` | 필수 |
| `@put` | Update | `Model.Method(args...)` | 필수 |
| `@delete` | Delete | `Model.Method(args...)` | 0개 시 WARNING |
| `@empty` | Guard: nil/zero → 404 | `target "message"` | — |
| `@exists` | Guard: not nil → 409 | `target "message"` | — |
| `@state` | State transition | `diagramID {inputs} "transition" "message"` | — |
| `@auth` | Permission check | `"action" "resource" {inputs} "message"` | — |
| `@call` | Function call | `[Type var =] package.Func(args...)` | — |
| `@response` | JSON response | `varName` or `{ field: var, ... }` | — |

`!` 접미사로 WARNING 억제: `@delete!`, `@response!`

### Args Format

`source.Field` or `"literal"`:
- `request.CourseID`, `course.InstructorID`, `currentUser.ID`, `config.APIKey`, `"cancelled"`

Reserved sources: `request`, `currentUser`, `config`, `query`

### Pagination

```go
// @get Page[Gig] gigPage = Gig.List({Query: query})      — offset pagination
// @get Cursor[Gig] gigCursor = Gig.List({Query: query})   — cursor pagination
// @get []Lesson lessons = Lesson.ListByCourse(request.CourseID)  — no pagination
```

`{Query: query}` → model method에 `opts QueryOpts` 파라미터 추가. `x-pagination` 있을 때만 사용.

| x-pagination | @get type | Model return |
|---|---|---|
| `offset` | `Page[T]` | `(*pagination.Page[T], error)` |
| `cursor` | `Cursor[T]` | `(*pagination.Cursor[T], error)` |
| 없음 | `[]T` or `T` | `([]T, error)` or `(*T, error)` |

### Package-Prefix @model (Non-DDL Models)

```go
// DDL 모델 (접두사 없음) — DDL 테이블이 SSOT
// @get User user = User.FindByID({ID: request.ID})

// 패키지 모델 (접두사 있음) — Go interface가 SSOT
// @get Session s = session.Session.Get({key: request.Token})
// @post CacheResult r = cache.Cache.Set({key: k, value: v, ttl: 300})
// @post FileResult r = file.File.Upload({key: path, body: request.File})
```

- 접두사 없음 → DDL 테이블 검증
- 접두사 있음 → Go interface 파싱 → 메서드/파라미터 검증
- `context.Context` 파라미터는 프레임워크 제공, SSaC에서 명시 불필요
- SSaC 파라미터명 = Go interface 파라미터명 (정확히 일치)

### Function Name = operationId

```
OpenAPI: operationId: EnrollCourse
SSaC:    func EnrollCourse()
STML:    data-action="EnrollCourse"
```

## Func Spec

`func/<pkg>/*.go`. 고정 시그니처: `func FuncName(req FuncNameRequest) (FuncNameResponse, error)`

### Purity Rule (I/O 금지)

`@call func`은 순수 로직만 허용. 금지 import: `database/sql`, `net/http`, `os`, `io`, `bufio` 등. I/O 필요 시 `@model` 사용.

### Fallback Chain

1. `specs/<project>/func/<pkg>/` — Project custom
2. `pkg/<pkg>/` — fullend default
3. Neither → ERROR with skeleton suggestion

## Built-in Functions (pkg/)

#### auth

| Function | Description |
|---|---|
| `hashPassword` | bcrypt 해싱 |
| `verifyPassword` | bcrypt 검증 (error=불일치) |
| `issueToken` | JWT 액세스 토큰 (24h) |
| `verifyToken` | JWT 검증 → claims |
| `refreshToken` | 리프레시 토큰 (7일) |
| `generateResetToken` | 비밀번호 리셋 랜덤 토큰 |

#### crypto

| Function | Description |
|---|---|
| `encrypt` / `decrypt` | AES-256-GCM |
| `generateOTP` / `verifyOTP` | TOTP |

#### storage

| Function | Description |
|---|---|
| `uploadFile` | S3 호환 업로드 |
| `deleteFile` | S3 호환 삭제 |
| `presignURL` | 서명된 다운로드 URL |

#### mail

| Function | Description |
|---|---|
| `sendEmail` | SMTP 평문 |
| `sendTemplateEmail` | Go 템플릿 HTML |

#### text

| Function | Description |
|---|---|
| `generateSlug` | URL-safe slug |
| `sanitizeHTML` | XSS 방지 |
| `truncateText` | 유니코드 안전 자르기 |

#### image

| Function | Description |
|---|---|
| `ogImage` | OG 이미지 (1200x630) |
| `thumbnail` | 썸네일 (200x200) |

## Built-in Models (pkg/)

패키지 접두사 @model로 사용. `fullend.yaml`에서 backend 설정.

#### session — 세션 (key-value + TTL)

```go
type SessionModel interface {
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Get(ctx context.Context, key string) (string, error)
    Delete(ctx context.Context, key string) error
}
```
구현체: PostgreSQL (`NewPostgresSession`), Memory (`NewMemorySession`)

#### cache — 캐시 (key-value + TTL)

SessionModel과 동일 interface. 목적만 다름 (데이터 효율화).
구현체: PostgreSQL (`NewPostgresCache`), Memory (`NewMemoryCache`)

#### file — 파일 스토리지

```go
type FileModel interface {
    Upload(ctx context.Context, key string, body io.Reader) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
}
```
구현체: S3 (`NewS3File`), LocalFile (`NewLocalFile`)

## Middleware — BearerAuth

`fullend.yaml` `backend.middleware`에 `bearerAuth` + OpenAPI `securitySchemes`에 `bearerAuth` 존재 시 gluegen이 `internal/middleware/bearerauth.go` 자동 생성.

- `Authorization: Bearer <token>` → `pkg/auth.VerifyToken` → `c.Set("currentUser", &model.CurrentUser{...})`
- 토큰 없거나 무효하면 빈 `CurrentUser{}` 세팅. `@auth`가 권한 검사 담당.
- `CurrentUser` struct는 `backend.auth.claims`에서 자동 생성 (`*_id` → `int64`, 나머지 → `string`)

## STML — UI Declarations

### Core data-* Attributes (8)

| Attribute | Value | Purpose |
|---|---|---|
| `data-fetch` | operationId | GET binding |
| `data-action` | operationId | POST/PUT/DELETE binding |
| `data-field` | field name | Request body field |
| `data-bind` | field name (dot) | Response field display |
| `data-param-*` | `route.ParamName` | Path/query parameter |
| `data-each` | array field name | List iteration |
| `data-state` | condition | Conditional rendering |
| `data-component` | component name | React component delegation |

### Infrastructure data-* Attributes (3)

| Attribute | Requirement |
|---|---|
| `data-paginate` | x-pagination in OpenAPI |
| `data-sort` | x-sort in OpenAPI (`column` or `column:desc`) |
| `data-filter` | x-filter in OpenAPI (`col1,col2`) |

### data-state Suffixes

`.empty` (array empty), `.loading` (loading), `.error` (error), plain (boolean field)

### custom.ts

`data-bind`가 OpenAPI response schema에 없는 필드를 참조할 때, `<page>.custom.ts`에서 동명 함수를 export하면 검증 통과.

## OpenAPI x- Extensions

```yaml
/courses:
  get:
    operationId: ListCourses
    x-pagination:
      style: offset           # offset | cursor
      defaultLimit: 20
      maxLimit: 100
    x-sort:
      allowed: [created_at, price]
      default: created_at
      direction: desc
    x-filter:
      allowed: [category, level]
    x-include:
      allowed: [instructor_id:users.id]   # FKColumn:RefTable.RefColumn
```

## sqlc Cardinality

| Cardinality | SSaC Type | Return |
|---|---|---|
| `:one` | `*Type` | `(*T, error)` |
| `:many` | `[]Type` | `([]T, error)` |
| `:exec` | (none) | `error` |

Model name from filename: `courses.sql` → `Course` (singular: `ies`→`y`, `sses`→`ss`, `xes`→`x`, else remove `s`)

## model/*.go

- `// @dto` → DDL 테이블 매칭 스킵 (순수 DTO: Token, Refund 등)
- `CurrentUser`는 `fullend.yaml` claims에서 자동 생성 — model/에 수동 생성 금지

## Mermaid stateDiagram

`states/*.md`. 파일명 = diagram ID. Transition label = SSaC 함수명 = operationId.

```markdown
# CourseState

​```mermaid
stateDiagram-v2
    [*] --> unpublished
    unpublished --> published: PublishCourse
    published --> deleted: DeleteCourse
​```
```

SSaC: `// @state course {status: course.Status} "PublishCourse" "상태 전이 불가"`

## OPA Rego

`policy/*.rego`. 5 allow patterns: unconditional, role-based, owner-based, role+owner, multiple actions.

### @ownership Annotations

```rego
# @ownership course: courses.instructor_id
# @ownership lesson: courses.instructor_id via lessons.course_id
# @ownership review: reviews.user_id
```

| Format | Meaning |
|---|---|
| `resource: table.column` | Direct lookup |
| `resource: table.column via join_table.fk` | JOIN lookup |

SSaC `@auth "action" "resource" {inputs} "message"` → Rego `input.action`/`input.resource` 매핑.

## Gherkin Scenario

`scenario/*.feature`. Tags: `@scenario` (비즈니스), `@invariant` (불변 검증).

### Action Steps

```
METHOD operationId {JSON} → result     # request + capture
METHOD operationId {JSON}              # request only
METHOD operationId → result            # no-body + capture
METHOD operationId                     # no-body only
```

`→ token` → Authorization header 자동 주입.

### Assertion Steps

```
status == CODE
response.field exists
response.field == value
response.array contains var.Field
response.array excludes var.Field
response.array count > N
```

## Name Matching Rules

| Source → Target | Matching |
|---|---|
| SSaC funcName ↔ OpenAPI operationId | Identical (PascalCase) |
| STML data-fetch/action ↔ OpenAPI operationId | Identical |
| stateDiagram transition ↔ SSaC funcName | Identical |
| SSaC Model (no prefix) ↔ DDL table | PascalCase → snake_case plural |
| SSaC Model.Method ↔ sqlc `-- name:` | Identical |
| SSaC @call pkg.Func ↔ Func spec | Identical |
| x-sort/filter allowed ↔ DDL column | Identical snake_case |

## Cross-Validation Rules

| Rule | Level |
|---|---|
| `backend.middleware` ↔ OpenAPI `securitySchemes` | ERROR |
| SSaC `currentUser` → `backend.auth.claims` 필수 | ERROR |
| SSaC `currentUser.X` → claims에 X 존재 | ERROR |
| SSaC `@auth` → claims 필수 | ERROR |
| x-sort/filter column ↔ DDL column 존재 | ERROR |
| x-sort column ↔ DDL index 존재 | WARNING |
| x-include ↔ DDL FK | WARNING |
| SSaC @result ↔ DDL table | WARNING |
| SSaC args ↔ DDL column | WARNING |
| SSaC funcName → operationId | ERROR |
| operationId → SSaC funcName | WARNING |
| States transition → SSaC funcName | ERROR |
| States transition → operationId | ERROR |
| SSaC @state → stateDiagram 존재 | ERROR |
| @state field → DDL column | ERROR |
| Policy ↔ SSaC @auth (action, resource) | WARNING |
| Policy @ownership → DDL table.column | ERROR |
| Policy @ownership via → DDL join FK | ERROR |
| Scenario operationId → OpenAPI | ERROR |
| Scenario METHOD → OpenAPI method | ERROR |
| Scenario JSON fields → request schema | ERROR |
| Scenario step order → States transitions | WARNING |
| Func → SSaC @call matching | ERROR |
| Func purity (I/O import 금지) | ERROR |
| Func body TODO stub | ERROR |
| Func arg count ↔ Request fields | ERROR |
| Func arg type ↔ Request field type | ERROR |
| DDL table → SSaC 참조 | WARNING |
| DDL column → OpenAPI schema | WARNING |
