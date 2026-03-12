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

### File Layout

- **One function per file.** Each `.ssac` file declares exactly one `func`.
- **Domain subdirectory required.** Files must be placed under `service/<domain>/`, not directly in `service/`.
  - Correct: `service/gig/create_gig.ssac`, `service/auth/login.ssac`
  - Wrong: `service/create_gig.ssac`

### File Extension: `.ssac`

Uses Go syntax but excluded from Go build via `.ssac` extension.

```go
package service

import "github.com/geul-org/fullend/pkg/auth"

// @call auth.HashPasswordResponse hp = auth.HashPassword({Password: request.password})
// @post User user = User.Create({Email: request.email, PasswordHash: hp.HashedPassword})
// @response { user: user }
func Register() {}
```

**Import declaration required.** When using `@call pkg.Func`, the package must be imported at the top of the file. Missing imports cause validation errors.

### 11 Sequence Types

| Type | Purpose | Format | Args |
|---|---|---|---|
| `@get` | Query | `Type var = Model.Method(args...)` | 0 args allowed |
| `@post` | Create | `Type var = Model.Method(args...)` | Required |
| `@put` | Update | `Model.Method(args...)` | Required |
| `@delete` | Delete | `Model.Method(args...)` | 0 args = WARNING |
| `@empty` | Guard: nil/zero → 404 | `target "message" [STATUS]` | STATUS: custom HTTP code (default 404) |
| `@exists` | Guard: not nil → 409 | `target "message" [STATUS]` | STATUS: custom HTTP code (default 409) |
| `@state` | State transition | `diagramID {inputs} "transition" "message" [STATUS]` | STATUS: custom HTTP code (default 409) |
| `@auth` | Permission check | `"action" "resource" {inputs} "message" [STATUS]` | STATUS: custom HTTP code (default 403) |
| `@call` | Function call | `[Type var =] package.Func(args...)` | — |
| `@publish` | Queue publish | `"topic" {payload} [{options}]` | — |
| `@response` | JSON response | `varName` or `{ field: var, ... }` | — |

### @subscribe Trigger

Executes a function when a queue event is received. Separate from HTTP triggers.

```go
// @subscribe "topic"
func OnEvent(message MessageType) {}
```

- Specify message type in function parameter (variable name must be `message`)
- Message struct must be declared as a Go struct in the same .ssac file
- Cannot use `@response` or `request`

**`@put` does not return a value.** To use the updated record in `@response`, re-query with `@get` after `@put`:

```go
// @put Gig.UpdateStatus({ID: gig.ID, Status: "published"})
// @get Gig updated = Gig.FindByID({ID: gig.ID})
// @response { gig: updated }
```

Append `!` to suppress WARNINGs: `@delete!`, `@response!`

### Args Format

`source.Field` or `"literal"`:
- `request.course_id`, `course.InstructorID`, `currentUser.ID`, `"cancelled"`

Reserved sources: `request`, `currentUser`, `query`, `message` (subscribe only)

#### request.* Field Case Rule

`request.*` field names must **exactly match the OpenAPI request schema property names**.
If OpenAPI uses snake_case, SSaC must use snake_case. If OpenAPI uses camelCase, SSaC must use camelCase.

```yaml
# OpenAPI schema
properties:
  bid_amount:
    type: integer
  email:
    type: string
```

```go
// SSaC — request.* uses the exact OpenAPI property name
// @post Proposal p = Proposal.Create({BidAmount: request.bid_amount})
// @call auth.HashPassword({Password: request.password})
```

**Note:** Sources other than `request.*` (model variables, currentUser, etc.) use Go PascalCase as-is.
- `request.email` (OpenAPI field name) vs `user.Email` (Go struct field name)

> **`config.*` forbidden**: Environment variables must not be passed via SSaC. Funcs read their own config via `os.Getenv()`.

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

**OpenAPI response for Page[T]:** Declare only `items` (array) and `total` (integer) in the response schema. `limit`/`offset` are handled automatically by the pagination framework — do not include them in the OpenAPI response.

```yaml
responses:
  200:
    content:
      application/json:
        schema:
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/Gig'
            total:
              type: integer
```

### Package-Prefix @model (Non-DDL Models)

```go
// DDL model (no prefix) — DDL table is SSOT
// @get User user = User.FindByID({ID: request.id})

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

### @func Annotation

Place `// @func camelCaseName` comment above the function. The camelCase name must match the SSaC `@call` reference.

```go
// @func holdEscrow
// @description Simulates locking funds in escrow

type HoldEscrowRequest struct {
    GigID    int64
    Amount   int64
    ClientID int64
}

type HoldEscrowResponse struct {
    TransactionID int64
}

func HoldEscrow(req HoldEscrowRequest) (HoldEscrowResponse, error) {
    return HoldEscrowResponse{TransactionID: req.GigID}, nil
}
```

SSaC reference: `// @call billing.HoldEscrowResponse r = billing.HoldEscrow({GigID: gig.ID, Amount: gig.Budget, ClientID: gig.ClientID})`

### Purity Rule

`@call func` allows only computation/judgment logic. Forbidden imports: `database/sql`, `net/http`, `io`, `bufio`, etc. Use `@model` for DB/file I/O.
`os` is allowed (for `os.Getenv()` — func reads its own config).

### Fallback Chain

1. `specs/<project>/func/<pkg>/` — Project custom
2. `pkg/<pkg>/` — fullend default
3. Neither → ERROR with skeleton suggestion

## Built-in Functions (pkg/)

#### auth

| Function | Request Fields | Response Fields |
|---|---|---|
| `hashPassword` | `Password` | `HashedPassword` |
| `verifyPassword` | `PasswordHash`, `Password` | (none) |
| `issueToken` | `UserID`, `Email`, `Role` | `AccessToken` |
| `verifyToken` | `Token`, `Secret` | `UserID`, `Email`, `Role` |
| `refreshToken` | `UserID`, `Email`, `Role` | `RefreshToken` |
| `generateResetToken` | (none) | `Token` |

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

| Function | Request Fields | Response Fields |
|---|---|---|
| `sendEmail` | `Host`, `Port`, `Username`, `Password`, `From`, `To`, `Subject`, `Body` | (none) |
| `sendTemplateEmail` | `To`, `Subject`, `TemplateName` | (none) |

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

Singleton package-level API. Reads OPA policy from `OPA_POLICY_PATH` environment variable at runtime.

```go
func Init(db *sql.DB, ownerships []OwnershipMapping) error
func Check(req CheckRequest) (CheckResponse, error)

type CheckRequest struct {
    Action     string
    Resource   string
    UserID     int64
    Role       string
    ResourceID int64
}

type OwnershipMapping struct {
    Resource string // "gig", "proposal"
    Table    string // "gigs", "proposals"
    Column   string // "client_id", "freelancer_id"
}
```

- SSaC `@auth` generates `authz.Check(authz.CheckRequest{...})` calls
- `Role: currentUser.Role` is auto-injected when `@auth` uses `currentUser`
- `authz.Init(conn, ownerships)` is auto-generated in `main.go` with ownership mappings from Rego `@ownership` annotations
- OPA input structure: `input.claims.user_id`, `input.claims.role`, `input.action`, `input.resource`, `input.resource_id`
- `data.owners` is loaded from DB per request based on `@ownership` mappings
- Set `OPA_POLICY_PATH` to the `.rego` file path (required unless `DISABLE_AUTHZ=1`)
- Set `DISABLE_AUTHZ=1` to bypass checks

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
// @call mail.SendEmail({To: message.Email, Subject: "Order completed"})
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

### sqlc Query Names and ModelPrefix

sqlc uses a **global namespace**, so `-- name:` values must be unique across all query files.
When multiple models have the same method name (Create, FindByID, etc.), **add a ModelPrefix** to disambiguate.

```sql
-- db/queries/users.sql
-- name: UserCreate :one
-- name: UserFindByID :one
-- name: UserFindByEmail :one

-- db/queries/gigs.sql
-- name: GigCreate :one
-- name: GigFindByID :one
-- name: GigList :many
-- name: GigUpdateStatus :exec
```

**In SSaC, the prefix is automatically stripped.** The `stripModelPrefix()` function removes the model name prefix from query names before registering them as methods.

| sqlc `-- name:` | Query file | Model | SSaC method name |
|---|---|---|---|
| `UserCreate` | `users.sql` | `User` | `Create` |
| `UserFindByID` | `users.sql` | `User` | `FindByID` |
| `GigCreate` | `gigs.sql` | `Gig` | `Create` |
| `GigUpdateStatus` | `gigs.sql` | `Gig` | `UpdateStatus` |

```go
// SSaC — call without prefix
// @post User user = User.Create({...})            ← sqlc: UserCreate
// @get Gig gig = Gig.FindByID({ID: request.id})   ← sqlc: GigFindByID
```

**Rule:** The ModelPrefix must exactly match the model name, and the character immediately after must be uppercase for stripping to occur.
`UserCreate` → `Create` (stripped), `Usercreate` → `Usercreate` (NOT stripped — next char is lowercase)

## model/*.go

- **Directory required.** Even if there are no `@dto` types, `model/model.go` must exist with at least a `package model` declaration. The codegen imports this package unconditionally.
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

`policy/*.rego`. Uses **OPA v1 syntax** — `if` keyword is required in all rules.

```rego
# OPA v1 (correct)
allow if {
    input.action == "CreateGig"
    input.resource == "gig"
}

# OPA v0 (wrong — will not parse)
allow {
    input.action == "CreateGig"
}
```

5 allow patterns: unconditional, role-based, owner-based, role+owner, multiple actions.

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

### Writing allow Rules

**Every allow rule must specify both `input.action` and `input.resource`.**
Crosscheck validates that SSaC `@auth "action" "resource"` pairs match Rego allow rule `input.action`/`input.resource` pairs.
Omitting `input.resource` causes the crosscheck to determine there is no matching rule for that action, resulting in an ERROR.

```rego
# Correct — both action and resource specified
allow if {
    input.action == "PublishCourse"
    input.resource == "course"
    input.claims.role == "instructor"
    data.owners.course[input.resource_id] == input.claims.user_id
}

# Wrong — input.resource missing → crosscheck ERROR
allow if {
    input.action == "PublishCourse"
    input.claims.role == "instructor"
    data.owners.course[input.resource_id] == input.claims.user_id
}
```

5 allow patterns:

| Pattern | Conditions |
|---|---|
| unconditional | `input.action` + `input.resource` only |
| role-based | + `input.claims.role` |
| owner-based | + `data.owners.resource[input.resource_id] == input.claims.user_id` |
| role+owner | both role + owner |
| multiple actions | multiple actions in same rule using `{...}` set |

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
| SSaC Model.Method ↔ sqlc `-- name:` | Identical (after ModelPrefix stripping) |
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
| SSaC @response field → OpenAPI response schema | ERROR |
| OpenAPI response field → SSaC @response | WARNING |
| `@publish` topic → `@subscribe` exists | WARNING |
| `@subscribe` topic → `@publish` exists | WARNING |
| `@subscribe` message fields → `@publish` payload fields | WARNING |
| `@publish`/`@subscribe` used → `queue.backend` required | ERROR |
| `@auth` inputs → authz CheckRequest fields | ERROR |
| `@empty/@exists/@state/@auth` ErrStatus → OpenAPI response defined | ERROR |
| FK + `DEFAULT 0` → sentinel record required in target table | WARNING |

## DDL Authoring Guide

### Go Reserved Words as Column Names

DDL column names that are Go reserved words (`type`, `range`, `select`, `map`, etc.) cause sqlc-generated code to fail compilation. Rename these columns:

| Avoid | Use Instead |
|---|---|
| `type` | `tx_type`, `gig_type`, `user_type` |
| `range` | `date_range`, `price_range` |
| `select` | `selected`, `selection` |

### FK DEFAULT 0 Pattern (Sentinel Record)

To avoid nullable FKs, use `NOT NULL DEFAULT 0`. In this case, the referenced table **must have an id=0 sentinel record**. Otherwise, FK constraint violations will cause INSERT failures.

```sql
-- gigs.freelancer_id: unassigned at creation → DEFAULT 0
freelancer_id BIGINT NOT NULL DEFAULT 0 REFERENCES users(id)
```

When using this pattern, add a sentinel to the referenced table's DDL:

```sql
-- Append to users.sql
INSERT INTO users (id, email, password_hash, role, name)
VALUES (0, 'nobody@system', '', 'system', 'Nobody')
ON CONFLICT DO NOTHING;
```

**Advantage:** Go struct stays `int64` — no `*int64`/`sql.NullInt64` needed, no nil checks.
**Caution:** Missing sentinel record causes FK violation errors. `fullend validate` detects this pattern and shows a WARNING.
