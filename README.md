# fullend

Full-stack SSOT orchestrator — validates consistency across 5 SSOT sources (STML, OpenAPI, SSaC, SQL DDL, Terraform) and generates code from them in a single CLI.

```
specs/
├── api/openapi.yaml       → OpenAPI 3.x
├── db/*.sql               → SQL DDL + sqlc queries
├── service/*.go           → SSaC (comment DSL)
├── model/*.go             → Go interfaces
├── frontend/*.html        → STML (HTML5 + data-*)
└── terraform/*.tf         → HCL
```

## Install

```bash
go install github.com/geul-org/fullend/artifacts/cmd/fullend@latest
```

## Commands

### validate

Validates each SSOT individually, then cross-validates consistency between layers.

```bash
fullend validate <specs-dir>
```

```
✓ OpenAPI      7 endpoints
✓ DDL          3 tables, 18 columns
✓ SSaC         7 service functions
✓ STML         4 pages, 6 bindings
✓ Cross        0 mismatches

All SSOT sources are consistent.
```

### gen

Validates first, then generates code from all SSOTs.

```bash
fullend gen <specs-dir> <artifacts-dir>
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
```

## Cross-Validation

Individual tools (SSaC, STML) validate within their own layer. fullend catches mismatches **between** layers:

- **OpenAPI x-sort/x-filter ↔ DDL** — referenced columns exist in tables
- **OpenAPI x-include ↔ DDL** — referenced resources map to tables
- **SSaC @result ↔ DDL** — result types match DDL-derived models
- **SSaC @param ↔ DDL** — parameter names match table columns
- **STML ↔ SSaC** (indirect) — both reference the same OpenAPI operationIds

## Related Projects

- [SSaC](https://github.com/geul-org/ssac) — Service Sequences as Code
- [STML](https://github.com/geul-org/stml) — Semantic Template Markup Language

## License

MIT — see [LICENSE](LICENSE).
