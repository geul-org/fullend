# fullend — AI SSOT Integration Guide

> Rules for writing 7 SSOTs (OpenAPI, SQL DDL, SSaC, STML, Mermaid stateDiagram, OPA Rego, Terraform) in a single project.
> Does not explain OpenAPI/SQL DDL/Terraform syntax. Covers only SSaC/STML/stateDiagram/OPA Rego syntax and cross-SSOT connection rules.

## Project Directory Structure

```
<project-root>/
├── api/openapi.yaml              # OpenAPI 3.x (with x- extensions)
├── db/
│   ├── *.sql                     # DDL (CREATE TABLE, CREATE INDEX)
│   └── queries/*.sql             # sqlc queries (-- name: Method :cardinality)
├── service/*.go                  # SSaC declarations (Go comment DSL)
├── model/*.go                    # Go interfaces (component, func definitions)
├── states/*.md                   # Mermaid stateDiagram (state transitions)
├── policy/*.rego                 # OPA Rego (authorization policies)
├── frontend/
│   ├── *.html                    # STML declarations (HTML5 + data-*)
│   ├── *.custom.ts               # Frontend computed functions (optional)
│   └── components/*.tsx          # React component wrappers (optional)
└── terraform/*.tf                # HCL infrastructure declarations
```

## SSaC — Service Logic Declarations

### Syntax

```go
// @sequence <type>        — block start
// @model <Model.Method>   — resource model.method
// @param <Name> <source> [-> column]  — source: request | currentUser | varName | "literal". -> column: explicit DDL column mapping
// @result <var> <Type>    — result binding
// @message "msg"          — custom error message (optional)
// @var <name>             — variable to return in response
// @action @resource @id   — authorize only (all 3 required)
// @component | @func      — call only (one required)
```

### 11 Sequence Types

| Type | Purpose | Required Tags |
|---|---|---|
| authorize | Permission check | @action, @resource, @id |
| get | Single/list query | @model, @result |
| guard nil | Return error if nil | target variable name |
| guard exists | Return error if not nil | target variable name |
| guard state | Check state transition validity | target stateDiagramID, @param entity.Field |
| post | Create | @model, @result |
| put | Update | @model |
| delete | Delete | @model |
| password | Password verification | @param x2 (hash, plain) |
| call | External component/function call | @component or @func |
| response | JSON response return | (none, @var optional) |

### Example: All Sequence Types

```go
// @sequence authorize
// @action update
// @resource course
// @id CourseID
//
// @sequence get
// @model Course.FindByID
// @param CourseID request
// @result course Course
//
// @sequence guard nil course
// @message "Course not found"
//
// @sequence get
// @model Enrollment.FindByCourseAndUser
// @param CourseID request
// @param UserID currentUser
// @result existing Enrollment
//
// @sequence guard exists existing
// @message "Already enrolled"
//
// @sequence password
// @param user.PasswordHash
// @param Password request
//
// @sequence post
// @model Enrollment.Create
// @param CourseID request
// @param UserID currentUser
// @result enrollment Enrollment
//
// @sequence put
// @model Course.IncrementEnrollCount
// @param CourseID request
//
// @sequence call
// @component notification
// @param enrollment
//
// @sequence response json
// @var enrollment
func EnrollCourse(w http.ResponseWriter, r *http.Request) {}
```

### @param Source Rules

| Source | Meaning | Codegen |
|---|---|---|
| `request` | HTTP request body/query | `r.FormValue("Name")` |
| `currentUser` | Authenticated user info | `currentUser.Name` |
| variable name | @result variable from previous sequence | Direct reference |
| `"literal"` | Hardcoded string | Used as-is |

`-> column` mapping: `@param PaymentMethod request -> method` — explicit DDL column mapping instead of auto snake_case conversion.

### Function Name = operationId

SSaC function names must match OpenAPI operationId exactly. This is the key that connects frontend (STML) and backend (SSaC).

```
OpenAPI: operationId: EnrollCourse
SSaC:    func EnrollCourse(...)
STML:    data-action="EnrollCourse"
```

## STML — UI Declarations

### Core data-* Attributes (8)

| Attribute | Value | Purpose | Placement |
|---|---|---|---|
| `data-fetch` | operationId | GET binding | Container element |
| `data-action` | operationId | POST/PUT/DELETE binding | form/button element |
| `data-field` | field name | Request body field | Inside data-action |
| `data-bind` | field name (dot notation) | Response field display | Inside data-fetch |
| `data-param-*` | `route.ParamName` | Path/query parameter | data-fetch or data-action element |
| `data-each` | array field name | List iteration | Inside data-fetch |
| `data-state` | condition expression | Conditional rendering | Anywhere |
| `data-component` | component name | React component delegation | Anywhere |

### Infrastructure data-* Attributes (3)

| Attribute | Value | Requirement |
|---|---|---|
| `data-paginate` | (no value, boolean) | Requires x-pagination in OpenAPI |
| `data-sort` | `column` or `column:desc` | Requires x-sort in OpenAPI |
| `data-filter` | `col1,col2` | Requires x-filter in OpenAPI |

### data-state Suffix Rules

| Pattern | Meaning | Codegen |
|---|---|---|
| `items.empty` | Array is empty | `{data.items?.length === 0 && ...}` |
| `items.loading` | Loading | `{isLoading && ...}` |
| `items.error` | Error occurred | `{isError && ...}` |
| `canEdit` | Boolean field | `{data.canEdit && ...}` |

### custom.ts Rules

When data-bind references a field not in the OpenAPI response schema, exporting a function with the same name in `<page>.custom.ts` passes validation.

```ts
// login-page.custom.ts
export function formattedDate(data) {
  return new Date(data.CreatedAt).toLocaleDateString()
}
```

### Example: Complex Page

```html
<main>
  <section data-fetch="ListCourses" data-paginate data-sort="created_at:desc" data-filter="category,level">
    <ul data-each="courses">
      <li>
        <h3 data-bind="title"></h3>
        <span data-bind="price"></span>
        <div data-component="RatingStars" data-bind="averageRating"></div>
      </li>
    </ul>
    <p data-state="courses.empty">No courses found</p>
    <div data-state="courses.loading">Loading...</div>
  </section>

  <form data-action="CreateCourse">
    <input data-field="Title" placeholder="Course title" />
    <input data-field="Price" type="number" placeholder="Price" />
    <select data-field="Category">
      <option value="dev">Development</option>
      <option value="design">Design</option>
    </select>
    <button type="submit">Create Course</button>
  </form>
</main>
```

## OpenAPI x- Extensions

Declare infrastructure parameters on OpenAPI endpoints. SSaC specs declare only business parameters; infrastructure parameters go in x- extensions only.

```yaml
/courses:
  get:
    operationId: ListCourses
    x-pagination:
      style: offset           # offset | cursor
      defaultLimit: 20
      maxLimit: 100
    x-sort:
      allowed: [created_at, price, rating]
      default: created_at
      direction: desc          # asc | desc
    x-filter:
      allowed: [category, level, instructor_id]
    x-include:
      allowed: [instructor_id:users.id]   # FKColumn:RefTable.RefColumn
```

### x-pagination

| Field | Type | Description |
|---|---|---|
| `style` | string | `offset` (Limit/Offset) or `cursor` (cursor-based) |
| `defaultLimit` | int | Default page size |
| `maxLimit` | int | Maximum page size |

### x-sort

| Field | Type | Description |
|---|---|---|
| `allowed` | string[] | Sortable columns (snake_case) |
| `default` | string | Default sort column |
| `direction` | string | `asc` or `desc` |

### x-filter

| Field | Type | Description |
|---|---|---|
| `allowed` | string[] | Filterable columns (snake_case) |

### x-include

| Field | Type | Description |
|---|---|---|
| `allowed` | string[] | Forward FK includes. Format: `FKColumn:RefTable.RefColumn` |

Syntax (single format only):
- `instructor_id:users.id` — courses.instructor_id -> users.id FK relation to include User
- Runtime include name: Remove `_id` from FK column (`instructor_id` -> `instructor`)
- Reverse FK (1:N) not supported — use separate endpoints

### x- Extension Codegen Effects

- SSaC: Operations with x- extensions get `opts QueryOpts` parameter auto-added to model methods
- SSaC: `:many` + x-pagination -> return type `([]T, int, error)` (includes total count)
- STML: `data-paginate` -> `useState(page, limit)` + prev/next buttons
- STML: `data-sort` -> `useState(sortBy, sortDir)` + toggle buttons
- STML: `data-filter` -> `useState(filters)` + filter inputs

## sqlc Query Rules

```sql
-- name: FindByID :one
SELECT * FROM courses WHERE id = $1;

-- name: List :many
SELECT * FROM courses ORDER BY created_at DESC;

-- name: Create :one
INSERT INTO courses (title, price, instructor_id)
VALUES ($1, $2, $3) RETURNING *;

-- name: SoftDelete :exec
UPDATE courses SET deleted_at = NOW() WHERE id = $1;
```

| Cardinality | SSaC @result Type | Codegen Return |
|---|---|---|
| `:one` | `*Type` | `(*Course, error)` |
| `:many` | `[]Type` | `([]Course, error)` |
| `:exec` | (none) | `error` |

Model name derived from sqlc query filename: `courses.sql` -> `Course`
Singularization rules: `ies`->`y`, `sses`->`ss`, `xes`->`x`, otherwise remove trailing `s`

## model/*.go Rules

Defines reference targets for SSaC `@component` and `@func`.

```go
// model/notification.go
package model

// NotificationService is a notification component.
type NotificationService interface {
    Send(userID int64, message string) error
}
```

- `type XxxInterface interface` -> referenceable via `@component xxx`
- `func Xxx(...)` -> referenceable via `@func xxx`
- Structs with `// @dto` comment -> skip DDL table matching (for pure DTOs like Token, Refund)

## SSOT Connection Map

```
         OpenAPI (operationId)
           |               |
    SSaC (funcName)    STML (data-fetch/action)
      |         |          |
  DDL (tables)  States   Policy (OPA Rego)
      |
  sqlc queries (model.method)
```

### Name Matching Rules

| Source | Target | Matching |
|---|---|---|
| stateDiagram transition event | SSaC funcName / OpenAPI operationId | Identical (PascalCase) |
| SSaC function name | OpenAPI operationId | Identical (PascalCase) |
| STML data-fetch/action | OpenAPI operationId | Identical (PascalCase) |
| SSaC @model Model | DDL table name | PascalCase -> snake_case + plural (`Course` -> `courses`) |
| SSaC @model .Method | sqlc query `-- name:` | Identical (`FindByID` = `FindByID`) |
| x-sort/filter allowed | DDL column name | Identical snake_case |
| x-include allowed | DDL FK relation | `FKColumn:RefTable.RefColumn` -> DDL FK mapping |

## fullend Cross-Validation Rules

After individual tools (ssac validate, stml validate) run their own checks, fullend catches cross-layer mismatches:

| Rule | Validation | Level |
|---|---|---|
| x-sort <-> DDL | Column exists in table | ERROR |
| x-sort <-> DDL index | Column has an index | WARNING |
| x-filter <-> DDL | Column exists in table | ERROR |
| x-include <-> DDL FK | Tables connected by FK relation | WARNING |
| SSaC @result <-> DDL | Result type has corresponding table | WARNING |
| SSaC @param <-> DDL | Parameter has corresponding column | WARNING |
| SSaC funcName -> operationId | SSaC function has corresponding operationId | ERROR |
| operationId -> SSaC funcName | operationId has corresponding SSaC function | WARNING |
| States transition -> SSaC | Transition event has corresponding SSaC function | ERROR |
| States transition -> OpenAPI | Transition event has corresponding operationId | ERROR |
| SSaC guard state -> States | Referenced stateDiagram exists | ERROR |
| States transition -> SSaC guard state | Function with transition has guard state | WARNING |
| guard state field -> DDL | State field exists as DDL column | ERROR |
| Policy <-> SSaC authorize | SSaC authorize (action, resource) -> Rego allow rule exists | WARNING |
| Policy <-> SSaC authorize | Rego allow (action, resource) -> SSaC authorize exists | WARNING |
| Policy @ownership -> DDL | @ownership table.column exists in DDL | ERROR |
| Policy @ownership via -> DDL | via join table.fk exists in DDL | ERROR |
| Policy <-> States | Transition event with authorize -> Rego allow rule exists | WARNING |

## Mermaid stateDiagram — State Transition Declarations

`states/*.md` files declare resource state transitions using Mermaid stateDiagram.

### Syntax

```markdown
# CourseState

​```mermaid
stateDiagram-v2
    [*] --> unpublished
    unpublished --> published: PublishCourse
    published --> deleted: DeleteCourse
    unpublished --> deleted: DeleteCourse
​```
```

### Rules

- Filename = stateDiagram ID (e.g., `course.md` -> `course`)
- Transition label = SSaC function name = OpenAPI operationId (PascalCase)
- `[*]` -> initial state (must match DDL DEFAULT value)
- One stateDiagram per file

### Usage in SSaC

```go
// @sequence guard state course
// @param course.Published
```

- `course`: stateDiagram ID (`states/course.md`)
- `course.Published`: State field from previous @result variable
- Function name is used as transition event (PublishCourse function -> PublishCourse transition)

### Codegen Output

```go
// guard state -> 409 Conflict if transition not allowed
if !coursestate.CanTransition(course.Published, "PublishCourse") {
    http.Error(w, "state transition not allowed", http.StatusConflict)
    return
}
```

State machine package (`states/<id>state/<id>state.go`) is auto-generated by fullend gen.

## OPA Rego — Authorization Policy Declarations

`policy/*.rego` files declare authorization policies using OPA Rego. fullend parses, cross-validates, and auto-generates an OPA Go SDK-based Authorizer implementation.

### Input Schema (Fixed)

| Field | Source | Description |
|---|---|---|
| `input.user.id` | `CurrentUser.UserID` | Authenticated user ID |
| `input.user.role` | `CurrentUser.Role` | User role |
| `input.action` | SSaC `@action` | Action to perform |
| `input.resource` | SSaC `@resource` | Target resource |
| `input.resource_id` | SSaC `@id` | Resource identifier |
| `input.resource_owner` | @ownership -> DB lookup | Resource owner ID |

### 5 Allow Patterns

```rego
# 1. Unconditional (authenticated only)
allow if { input.action == "create"; input.resource == "course" }

# 2. Role-based
allow if { input.action == "create"; input.resource == "course"; input.user.role == "instructor" }

# 3. Owner-based
allow if { input.action == "update"; input.resource == "course"; input.user.id == input.resource_owner }

# 4. Role + Owner
allow if { input.action == "delete"; input.resource == "course"; input.user.role == "instructor"; input.user.id == input.resource_owner }

# 5. Multiple actions (set)
allow if { input.action in {"update", "delete", "publish"}; input.resource == "course"; input.user.id == input.resource_owner }
```

### @ownership Annotations

Declare owner lookup methods at the top of Rego files:

```rego
# @ownership course: courses.instructor_id
# @ownership lesson: courses.instructor_id via lessons.course_id
# @ownership review: reviews.user_id
```

| Format | Meaning |
|---|---|
| `resource: table.column` | Direct lookup |
| `resource: table.column via join_table.fk` | JOIN lookup |

### Relationship with SSaC authorize

SSaC `authorize` sequence `@action`/`@resource` maps to Rego `allow` rule `input.action`/`input.resource`. Independent concern from stateDiagram (state vs permission).

### Codegen Output

`authz/authz.go` — OPA Go SDK-based Authorizer implementation (evaluates embedded .rego file)
`authz/<name>.rego` — Copied from specs (for go:embed)

## Runtime Testing (Hurl)

`fullend gen` auto-generates Hurl smoke tests from OpenAPI specs.

```bash
# After starting the server:
hurl --test --variable host=http://localhost:8080 artifacts/<project>/tests/smoke.hurl
```

Generated content:
- Auth flow (Register -> Login -> token capture)
- Per-resource CRUD (POST -> GET -> PUT -> DELETE, dependency order)
- Response schema assertions (jsonpath)
- x-pagination/sort/filter/include parameter coverage

## fullend CLI

```bash
fullend validate [--skip kind,...] <specs-dir>                 # Individual validation + cross-validation
fullend gen      [--skip kind,...] <specs-dir> <artifacts-dir> # validate -> codegen + Hurl tests + state machines + OPA Authorizer
fullend status   <specs-dir>                                   # SSOT summary
```

All 8 SSOTs (OpenAPI, DDL, SSaC, Model, STML, States, Policy, Terraform) are **required by default**. Missing SSOTs cause an ERROR. Use `--skip` to explicitly exclude:

```bash
fullend validate --skip states,terraform specs/my-project
```

Skip kinds: `openapi`, `ddl`, `ssac`, `model`, `stml`, `states`, `policy`, `terraform`
