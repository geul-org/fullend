# fullend — AI SSOT Integration Guide

> Covers SSaC, STML, Func Spec, Mermaid stateDiagram, OPA Rego, Gherkin, OpenAPI x- extensions, cross-validation rules, and pkg/ functions/models.
> Does NOT explain OpenAPI/SQL DDL/Terraform syntax.

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

queue:
  backend: postgres                  # postgres | memory

authz:
  package: github.com/org/project/internal/authz  # custom authz package (optional)
```

### Required Fields

`apiVersion` (fullend/v1), `kind` (Project), `metadata.name`, `backend.module`

### Optional Fields

| Field | Description |
|---|---|
| `backend.auth.claims` | JWT claims → generates `CurrentUser` struct |
| `session.backend` | Session backend: `postgres` or `memory` |
| `cache.backend` | Cache backend: `postgres` or `memory` |
| `file.backend` | File storage: `s3` or `local` |
| `queue.backend` | Queue backend: `postgres` or `memory` |
| `authz.package` | Custom authz package path (default: `pkg/authz`) |

## SSaC — Service Logic Declarations

### File Extension: `.ssac`

Uses Go syntax but excluded from Go build via `.ssac` extension.

```go
package service

import "github.com/geul-org/fullend/pkg/auth"

// @call auth.HashPasswordResponse hp = auth.HashPassword({Password: request.Password})
// @post User user = User.Create({Email: request.Email, PasswordHash: hp.HashedPassword})
// @response { user: user }
func Register() {}
```

### 11 Sequence Types

| Type | Purpose | Format | Args |
|---|---|---|---|
| `@get` | Query | `Type var = Model.Method(args...)` | 0 args allowed |
| `@post` | Create | `Type var = Model.Method(args...)` | Required |
| `@put` | Update | `Model.Method(args...)` | Required |
| `@delete` | Delete | `Model.Method(args...)` | 0 args = WARNING |
| `@empty` | Guard: nil/zero → 404 | `target "message"` | — |
| `@exists` | Guard: not nil → 409 | `target "message"` | — |
| `@state` | State transition | `diagramID {inputs} "transition" "message"` | — |
| `@auth` | Permission check | `"action" "resource" {inputs} "message"` | — |
| `@call` | Function call | `[Type var =] package.Func(args...)` | — |
| `@publish` | Queue publish | `"topic" {payload} [{options}]` | — |
| `@response` | JSON response | `varName` or `{ field: var, ... }` | — |

### @subscribe Trigger

Queue 이벤트 수신 시 함수를 실행한다. HTTP 트리거와 별도.

```go
// @subscribe "topic"
func OnEvent(message MessageType) {}
```

- 함수 파라미터에 메시지 타입 명시 (변수명은 반드시 `message`)
- 메시지 struct는 같은 .ssac 파일에 Go struct로 선언
- `@response` 사용 불가, `request` 사용 불가

Append `!` to suppress WARNINGs: `@delete!`, `@response!`

### Args Format

`source.Field` or `"literal"`:
- `request.CourseID`, `course.InstructorID`, `currentUser.ID`, `"cancelled"`

Reserved sources: `request`, `currentUser`, `query`, `message` (subscribe only)

> **`config.*` 금지**: 환경 변수는 SSaC에서 전달하지 않는다. func 내부에서 직접 `os.Getenv()`로 읽는다.

### Pagination

```go
// @get Page[Gig] gigPage = Gig.List({Query: query})      — offset pagination
// @get Cursor[Gig] gigCursor = Gig.List({Query: query})   — cursor pagination
// @get []Lesson lessons = Lesson.ListByCourse(request.CourseID)  — no pagination
```

`{Query: query}` adds `opts QueryOpts` parameter to model method. Use only with `x-pagination`.

| x-pagination | @get type | Model return |
|---|---|---|
| `offset` | `Page[T]` | `(*pagination.Page[T], error)` |
| `cursor` | `Cursor[T]` | `(*pagination.Cursor[T], error)` |
| none | `[]T` or `T` | `([]T, error)` or `(*T, error)` |

### Package-Prefix @model (Non-DDL Models)

```go
// DDL model (no prefix) — DDL table is SSOT
// @get User user = User.FindByID({ID: request.ID})

// Package model (with prefix) — Go interface is SSOT
// @get Session s = session.Session.Get({key: request.Token})
// @post CacheResult r = cache.Cache.Set({key: k, value: v, ttl: 300})
// @post FileResult r = file.File.Upload({key: path, body: request.File})
```

- No prefix → DDL table validation
- With prefix → Go interface parsing → method/parameter validation
- `context.Context` parameter is framework-provided, omit from SSaC
- SSaC parameter names must exactly match Go interface parameter names

### Function Name = operationId

```
OpenAPI: operationId: EnrollCourse
SSaC:    func EnrollCourse()
STML:    data-action="EnrollCourse"
```

## Func Spec

`func/<pkg>/*.go`. Fixed signature: `func FuncName(req FuncNameRequest) (FuncNameResponse, error)`

### Purity Rule

`@call func` allows only computation/judgment logic. Forbidden imports: `database/sql`, `net/http`, `io`, `bufio`, etc. Use `@model` for DB/file I/O.
`os` is allowed (for `os.Getenv()` — func reads its own config).

### Fallback Chain

1. `specs/<project>/func/<pkg>/` — Project custom
2. `pkg/<pkg>/` — fullend default
3. Neither → ERROR with skeleton suggestion

## Built-in Functions (pkg/)

#### auth

| Function | Description |
|---|---|
| `hashPassword` | bcrypt hashing |
| `verifyPassword` | bcrypt verification (error = mismatch) |
| `issueToken` | JWT access token (24h) |
| `verifyToken` | JWT verification → claims |
| `refreshToken` | Refresh token (7 days) |
| `generateResetToken` | Random hex token for password reset |

#### crypto

| Function | Description |
|---|---|
| `encrypt` / `decrypt` | AES-256-GCM |
| `generateOTP` / `verifyOTP` | TOTP |

#### storage

| Function | Description |
|---|---|
| `uploadFile` | S3-compatible upload |
| `deleteFile` | S3-compatible deletion |
| `presignURL` | Presigned download URL |

#### mail

| Function | Description |
|---|---|
| `sendEmail` | SMTP plain text |
| `sendTemplateEmail` | Go template HTML |

#### text

| Function | Description |
|---|---|
| `generateSlug` | URL-safe slug |
| `sanitizeHTML` | XSS prevention |
| `truncateText` | Unicode-safe truncation |

#### image

| Function | Description |
|---|---|
| `ogImage` | OG image (1200x630) |
| `thumbnail` | Thumbnail (200x200) |

## Built-in Models (pkg/)

Used as package-prefix @model. Backend configured in `fullend.yaml`.

#### session — Session (key-value + TTL)

```go
type SessionModel interface {
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Get(ctx context.Context, key string) (string, error)
    Delete(ctx context.Context, key string) error
}
```
Backends: PostgreSQL (`NewPostgresSession`), Memory (`NewMemorySession`)

#### cache — Cache (key-value + TTL)

Same interface as SessionModel. Different purpose (data efficiency).
Backends: PostgreSQL (`NewPostgresCache`), Memory (`NewMemoryCache`)

#### file — File Storage

```go
type FileModel interface {
    Upload(ctx context.Context, key string, body io.Reader) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
}
```
Backends: S3 (`NewS3File`), LocalFile (`NewLocalFile`)

#### authz — Authorization (OPA Rego)

Singleton package-level API. Configured via `fullend.yaml` `authz.package` (default: `pkg/authz`).

```go
func Init(db *sql.DB) error
func Check(req CheckRequest) (CheckResponse, error)

type CheckRequest struct {
    Action     string
    Resource   string
    UserID     int64
    ResourceID int64
}
```

- SSaC `@auth` generates `authz.Check(authz.CheckRequest{...})` calls
- `authz.Init(conn)` is auto-generated in `main.go` when `@auth` is used
- Set `DISABLE_AUTHZ=1` to bypass checks
- Custom authz package: set `authz.package` in `fullend.yaml`

#### queue — Queue Pub/Sub

Singleton package-level API (not a model interface). Configured via `fullend.yaml` `queue.backend`.

```go
func Init(ctx context.Context, backend string, db *sql.DB) error
func Publish(ctx context.Context, topic string, payload any, opts ...PublishOption) error
func Subscribe(topic string, handler func(ctx context.Context, msg []byte) error)
func Start(ctx context.Context) error
func Close() error

func WithDelay(seconds int) PublishOption
func WithPriority(p string) PublishOption
```

Backends: PostgreSQL (`fullend_queue` table, polling), Memory (synchronous, test only)

SSaC usage:
```go
// @publish "order.completed" {OrderID: order.ID, Email: order.Email}
// @publish "cart.abandoned" {CartID: cart.ID} {delay: 1800}
```

Subscribe functions use `@subscribe` trigger with message struct:
```go
type OnOrderCompletedMessage struct {
    OrderID int64
    Email   string
}

// @subscribe "order.completed"
// @get Order order = Order.FindByID({ID: message.OrderID})
// @call mail.SendEmail({To: message.Email, Subject: "주문 완료"})
func OnOrderCompleted(message OnOrderCompletedMessage) {}
```

## Middleware — BearerAuth

Auto-generated when `backend.middleware` has `bearerAuth` + OpenAPI `securitySchemes` has `bearerAuth`.

- `Authorization: Bearer <token>` → `pkg/auth.VerifyToken` → sets `*model.CurrentUser` in gin context
- Missing/invalid token → sets empty `CurrentUser{}`. `@auth` handles permission checks.
- `CurrentUser` struct auto-generated from `backend.auth.claims` (`*_id` → `int64`, else → `string`)

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

When `data-bind` references a field not in the OpenAPI response schema, exporting a function with the same name in `<page>.custom.ts` passes validation.

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

- `// @dto` → Skip DDL table matching (pure DTOs: Token, Refund, etc.)
- `CurrentUser` is auto-generated from `fullend.yaml` claims — do NOT create manually in model/

## Mermaid stateDiagram

`states/*.md`. Filename = diagram ID. Transition label = SSaC function name = operationId.

```markdown
# CourseState

​```mermaid
stateDiagram-v2
    [*] --> unpublished
    unpublished --> published: PublishCourse
    published --> deleted: DeleteCourse
​```
```

SSaC: `// @state course {status: course.Status} "PublishCourse" "Cannot transition"`

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

SSaC `@auth "action" "resource" {inputs} "message"` maps to Rego `input.action`/`input.resource`.

@auth generates `authz.Check(authz.CheckRequest{...})` package function call (not a method on Handler).
Default authz package: `pkg/authz` (OPA Rego-based). Custom package via `fullend.yaml` `authz.package`.

## Gherkin Scenario

`scenario/*.feature`. Tags: `@scenario` (business), `@invariant` (invariant verification).

### Action Steps

```
METHOD operationId {JSON} → result     # request + capture
METHOD operationId {JSON}              # request only
METHOD operationId → result            # no-body + capture
METHOD operationId                     # no-body only
```

`→ token` auto-injects Authorization header.

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
| SSaC `currentUser` → `backend.auth.claims` required | ERROR |
| SSaC `currentUser.X` → X must exist in claims | ERROR |
| SSaC `@auth` → claims required | ERROR |
| x-sort/filter column ↔ DDL column exists | ERROR |
| x-sort column ↔ DDL index exists | WARNING |
| x-include ↔ DDL FK | WARNING |
| SSaC @result ↔ DDL table | WARNING |
| SSaC args ↔ DDL column | WARNING |
| SSaC funcName → operationId | ERROR |
| operationId → SSaC funcName | WARNING |
| States transition → SSaC funcName | ERROR |
| States transition → operationId | ERROR |
| SSaC @state → stateDiagram exists | ERROR |
| @state field → DDL column | ERROR |
| Policy ↔ SSaC @auth (action, resource) | WARNING |
| Policy @ownership → DDL table.column | ERROR |
| Policy @ownership via → DDL join FK | ERROR |
| Scenario operationId → OpenAPI | ERROR |
| Scenario METHOD → OpenAPI method | ERROR |
| Scenario JSON fields → request schema | ERROR |
| Scenario step order → States transitions | WARNING |
| Func → SSaC @call matching | ERROR |
| Func purity (I/O import forbidden, `os` allowed) | ERROR |
| Func body TODO stub | ERROR |
| Func arg count ↔ Request fields | ERROR |
| Func arg type ↔ Request field type | ERROR |
| DDL table → SSaC reference | ERROR |
| DDL column → OpenAPI schema | WARNING |
| `@publish` topic → `@subscribe` exists | WARNING |
| `@subscribe` topic → `@publish` exists | WARNING |
| `@subscribe` message fields → `@publish` payload fields | WARNING |
| `@publish`/`@subscribe` used → `queue.backend` required | ERROR |
| `@auth` inputs → authz CheckRequest fields | ERROR |
| FK + `DEFAULT 0` → 대상 테이블에 id=0 센티널 필요 | WARNING |

## DDL 작성 가이드

### FK DEFAULT 0 패턴 (센티널 레코드)

nullable FK를 피하고 `NOT NULL DEFAULT 0`을 사용할 경우, 참조 대상 테이블에 **id=0 센티널 레코드**를 반드시 삽입해야 한다. 그렇지 않으면 FK 제약 위반으로 INSERT가 실패한다.

```sql
-- gigs.freelancer_id: 생성 시점에 미배정 → DEFAULT 0
freelancer_id BIGINT NOT NULL DEFAULT 0 REFERENCES users(id)
```

이 패턴을 쓸 때 참조 대상 DDL에 센티널을 추가한다:

```sql
-- users.sql 끝에 추가
INSERT INTO users (id, email, password_hash, role, name)
VALUES (0, 'nobody@system', '', 'system', 'Nobody')
ON CONFLICT DO NOTHING;
```

**장점:** Go struct가 `int64` 유지, `*int64`/`sql.NullInt64` 불필요, nil 체크 코드 없음.
**주의:** 센티널 레코드 누락 시 FK 위반 에러 발생. fullend validate가 이 패턴을 감지하여 WARNING을 표시한다.
