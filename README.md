# fullend

Full-stack SSOT orchestrator — validates consistency across 7 SSOT sources (STML, OpenAPI, SSaC, SQL DDL, Mermaid stateDiagram, OPA Rego, Terraform) and generates code from them in a single CLI.

```
specs/
├── api/openapi.yaml       → OpenAPI 3.x
├── db/*.sql               → SQL DDL + sqlc queries
├── service/*.go           → SSaC (comment DSL)
├── model/*.go             → Go interfaces
├── states/*.md            → Mermaid stateDiagram (state transitions)
├── policy/*.rego          → OPA Rego (authorization policies)
├── frontend/*.html        → STML (HTML5 + data-*)
└── terraform/*.tf         → HCL
```

## Install

```bash
go install github.com/geul-org/fullend/artifacts/cmd/fullend@latest
```

## Commands

### validate

Validates each SSOT individually, then cross-validates consistency between layers. All 8 SSOTs are required by default; use `--skip` to exclude specific kinds.

```bash
fullend validate <specs-dir>
fullend validate --skip states,terraform <specs-dir>
```

```
✓ OpenAPI      7 endpoints
✓ DDL          3 tables, 18 columns
✓ SSaC         7 service functions
✓ Model        3 files
✓ STML         4 pages, 6 bindings
✓ States       1 diagrams, 3 transitions
✓ Policy       1 files, 5 rules, 3 ownership mappings
✓ Terraform    2 files
✓ Cross        0 mismatches

All SSOT sources are consistent.
```

Skip kinds: `openapi`, `ddl`, `ssac`, `model`, `stml`, `states`, `policy`, `terraform`

### gen

Validates first, then generates code from all SSOTs. Accepts the same `--skip` option.

```bash
fullend gen <specs-dir> <artifacts-dir>
fullend gen --skip terraform <specs-dir> <artifacts-dir>
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
```

## Cross-Validation

Individual tools (SSaC, STML) validate within their own layer. fullend catches mismatches **between** layers:

- **OpenAPI x-sort/x-filter ↔ DDL** — referenced columns exist in tables
- **OpenAPI x-include ↔ DDL** — referenced resources map to tables
- **SSaC @result ↔ DDL** — result types match DDL-derived models
- **SSaC @param ↔ DDL** — parameter names match table columns
- **States ↔ SSaC** — transition events match SSaC functions, guard state references valid diagrams
- **States ↔ DDL** — state fields map to existing DDL columns
- **States ↔ OpenAPI** — transition events match operationIds
- **Policy ↔ SSaC** — authorize (action, resource) pairs match Rego allow rules
- **Policy ↔ DDL** — @ownership table/column references exist in DDL
- **Policy ↔ States** — state transition events with authorize have matching Rego rules
- **STML ↔ SSaC** (indirect) — both reference the same OpenAPI operationIds

## Runtime Testing

`fullend gen` also generates [Hurl](https://hurl.dev) smoke test scenarios from OpenAPI specs.

```bash
# Start your server, then:
hurl --test --variable host=http://localhost:8080 artifacts/my-project/tests/smoke.hurl
```

Generated tests cover: auth flow, CRUD operations, response schema validation, pagination/sort/filter/include parameters.

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
- [lib/pq](https://github.com/lib/pq) — PostgreSQL driver for Go

## License

MIT — see [LICENSE](LICENSE).
