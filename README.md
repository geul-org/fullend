# fullend

Full-stack SSOT orchestrator — validates consistency across 10 SSOT sources (fullend.yaml, STML, OpenAPI, SSaC, SQL DDL, Mermaid stateDiagram, OPA Rego, Gherkin Scenario, Func Spec, Terraform) and generates code from them in a single CLI.

```
specs/
├── fullend.yaml             → Project config (required)
├── api/openapi.yaml         → OpenAPI 3.x
├── db/*.sql                 → SQL DDL + sqlc queries
├── service/**/*.ssac        → SSaC (comment DSL, .ssac extension)
├── model/*.go               → Go structs (// @dto for non-DDL types)
├── func/<pkg>/*.go          → Custom func implementations (optional)
├── states/*.md              → Mermaid stateDiagram (state transitions)
├── policy/*.rego            → OPA Rego (authorization policies)
├── scenario/*.feature       → Gherkin (business scenarios)
├── frontend/*.html          → STML (HTML5 + data-*)
└── terraform/*.tf           → HCL
```

## Install

```bash
go install github.com/geul-org/fullend/cmd/fullend@latest
```

## Commands

### validate

Validates each SSOT individually, then cross-validates consistency between layers. 10 SSOTs are required by default; Func is optional (only when `func/` exists). Use `--skip` to exclude specific kinds.

```bash
fullend validate <specs-dir>
fullend validate --skip states,terraform <specs-dir>
```

```
✓ Config       my-project, go/gin, typescript/react
✓ OpenAPI      7 endpoints
✓ DDL          3 tables, 18 columns
✓ SSaC         7 service functions
✓ Model        3 files
✓ STML         4 pages, 6 bindings
✓ States       1 diagrams, 3 transitions
✓ Policy       1 files, 5 rules, 3 ownership mappings
✓ Scenario     4 features, 5 scenarios
✓ Func         3 funcs
✓ Terraform    2 files
✓ Cross        0 mismatches

All SSOT sources are consistent.
```

Skip kinds: `openapi`, `ddl`, `ssac`, `model`, `stml`, `states`, `policy`, `scenario`, `func`, `terraform`

### gen

Validates first, then generates code from all SSOTs. Accepts the same `--skip` option.

```bash
fullend gen <specs-dir> <artifacts-dir>
fullend gen --skip terraform <specs-dir> <artifacts-dir>
```

### gen-model

Generates a Go model file (interface + types + HTTP client) from an external OpenAPI document. Accepts a local file path or URL.

```bash
fullend gen-model <openapi-source> <output-dir>
fullend gen-model https://api.stripe.com/openapi.yaml ./external/
fullend gen-model specs/my-project/external/escrow.openapi.yaml specs/my-project/external/
```

### status

Shows a summary of detected SSOTs and their stats.

```bash
fullend status <specs-dir>
```

```
SSOT Status:
  OpenAPI      api/openapi.yaml               7 endpoints
  DDL          db                             3 tables, 18 columns
  SSaC         service                        7 functions
  STML         frontend                       4 pages
  States       states                         1 diagrams, 3 transitions
  Policy       policy                         1 files, 5 rules
  Scenario     scenario                       4 features, 5 scenarios
  Func         func                           3 funcs
```

## Default Functions (pkg/)

fullend ships with built-in function implementations that can be used via SSaC `@call`:

| Package | Function | Description |
|---|---|---|
| `auth` | `hashPassword` | bcrypt password hashing |
| `auth` | `verifyPassword` | bcrypt password verification |
| `auth` | `issueToken` | JWT access token generation (24h) |
| `auth` | `verifyToken` | JWT token verification + claims extraction |
| `auth` | `refreshToken` | Refresh token generation (7 days) |
| `auth` | `generateResetToken` | Random hex token for password reset |
| `crypto` | `encrypt` | AES-256-GCM symmetric encryption |
| `crypto` | `decrypt` | AES-256-GCM decryption |
| `crypto` | `generateOTP` | TOTP secret + QR provisioning URL |
| `crypto` | `verifyOTP` | TOTP code verification |
| `storage` | `uploadFile` | S3-compatible file upload |
| `storage` | `deleteFile` | S3-compatible file deletion |
| `storage` | `presignURL` | S3 presigned download URL |
| `mail` | `sendEmail` | SMTP plain text email |
| `mail` | `sendTemplateEmail` | Go template HTML email via SMTP |
| `text` | `generateSlug` | Unicode to URL-safe slug |
| `text` | `sanitizeHTML` | XSS prevention HTML sanitization |
| `text` | `truncateText` | Unicode-safe text truncation |
| `image` | `ogImage` | OG image generation (1200x630, PNG) |
| `image` | `thumbnail` | Thumbnail generation (200x200, PNG) |

Projects can override these by providing custom implementations in `specs/<project>/func/<pkg>/`.

## Built-in Models (pkg/)

Package-prefix @model interfaces for non-DDL I/O. Configured via `fullend.yaml`.

| Package | Interface | Backends | SSaC Usage |
|---|---|---|---|
| `session` | `SessionModel` (Set/Get/Delete + TTL) | PostgreSQL, Memory | `session.Session.Get({key: ...})` |
| `cache` | `CacheModel` (Set/Get/Delete + TTL) | PostgreSQL, Memory | `cache.Cache.Set({key: ..., value: ..., ttl: ...})` |
| `file` | `FileModel` (Upload/Download/Delete) | S3, LocalFile | `file.File.Upload({key: ..., body: ...})` |
| `queue` | Singleton Pub/Sub (Publish/Subscribe) | PostgreSQL, Memory | `@publish "topic" {payload}` |

## Middleware (Generated)

gluegen generates project-specific `internal/middleware/bearerauth.go` from `fullend.yaml` claims config.

| Middleware | Trigger | Description |
|---|---|---|
| `BearerAuth(secret)` | `securitySchemes.bearerAuth` + `backend.auth.claims` | Extracts JWT → sets `*model.CurrentUser` in gin context |

Route grouping is determined by OpenAPI `security` field on each operation:
- Operations with `security: [{bearerAuth: []}]` → auth group (middleware applied)
- Operations without `security` → public group (no middleware)

## Cross-Validation

Individual tools (SSaC, STML) validate within their own layer. fullend catches mismatches **between** layers:

- **fullend.yaml ↔ OpenAPI** — middleware names match securitySchemes keys
- **OpenAPI x-sort/x-filter ↔ DDL** — referenced columns exist in tables
- **OpenAPI x-include ↔ DDL** — referenced resources map to tables
- **SSaC @result ↔ DDL** — result types match DDL-derived models
- **SSaC arg ↔ DDL** — arg field names match table columns
- **States ↔ SSaC** — transition events match SSaC functions, guard state references valid diagrams
- **States ↔ DDL** — state fields map to existing DDL columns
- **States ↔ OpenAPI** — transition events match operationIds
- **Policy ↔ SSaC** — @auth (action, resource) pairs match Rego allow rules
- **Policy ↔ DDL** — @ownership table/column references exist in DDL
- **Policy ↔ States** — state transition events with @auth have matching Rego rules
- **Scenario ↔ OpenAPI** — operationIds, methods, and request fields match
- **Scenario ↔ States** — step order follows state transition rules
- **Queue** — @publish topics match @subscribe functions, payload/message field consistency, queue config required
- **Func ↔ SSaC** — @call references have matching implementations, arg count matches Request fields, positional types match (via DDL/OpenAPI), result/response consistency
- **STML ↔ SSaC** (indirect) — both reference the same OpenAPI operationIds

## Runtime Testing

`fullend gen` generates [Hurl](https://hurl.dev) tests from OpenAPI specs and Gherkin scenarios.

```bash
# Start your server, then:
hurl --test --variable host=http://localhost:8080 artifacts/my-project/tests/*.hurl
```

Generated tests include:
- **smoke.hurl** — OpenAPI endpoint smoke tests (auto-generated)
- **scenario-*.hurl** — Business scenario tests (from .feature files)
- **invariant-*.hurl** — Cross-endpoint invariant tests (from .feature files)

## Related Projects

- [SSaC](https://github.com/geul-org/ssac) — Service Sequences as Code
- [STML](https://github.com/geul-org/stml) — Semantic Template Markup Language

## Acknowledgments

fullend is built on the shoulders of these projects. Without them, this tool would not exist.

### SSOT Foundations

These projects define the standards that fullend orchestrates. They are the reason fullend can exist as a single-CLI full-stack generator.

- [OpenAPI Initiative](https://www.openapis.org/) — The API description standard that connects frontend and backend
- [sqlc](https://sqlc.dev/) — SQL-first Go code generation. fullend's DDL-driven model approach is directly inspired by sqlc's philosophy
- [Open Policy Agent](https://www.openpolicyagent.org/) — Policy as code. OPA's Rego language powers fullend's authorization layer
- [Mermaid](https://mermaid.js.org/) — Diagram as code. State diagrams become runtime-enforceable state machines
- [Terraform](https://www.terraform.io/) — Infrastructure as code. The original declarative infrastructure standard

### Code Generation & Validation

- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) — OpenAPI to Go server/types code generation
- [kin-openapi](https://github.com/getkin/kin-openapi) — OpenAPI 3.x parsing and validation in Go
- [Hurl](https://hurl.dev/) — Plain-text HTTP testing. fullend generates Hurl smoke tests from OpenAPI specs

### Generated Code Runtime

Projects that fullend's generated code depends on at runtime:

- [React](https://react.dev/) — UI library for generated frontend
- [React Router](https://reactrouter.com/) — Client-side routing
- [TanStack Query](https://tanstack.com/query) — Data fetching and caching
- [React Hook Form](https://react-hook-form.com/) — Form state management
- [Vite](https://vite.dev/) — Frontend build tool
- [Tailwind CSS](https://tailwindcss.com/) — Utility-first CSS framework
- [TypeScript](https://www.typescriptlang.org/) — Type-safe JavaScript
- [Gin](https://gin-gonic.com/) — HTTP web framework for Go
- [lib/pq](https://github.com/lib/pq) — PostgreSQL driver for Go

## License

MIT — see [LICENSE](LICENSE).
