# fullend SSOT Agent Instructions

## 1. Read the SSOT Manual First

Before starting, read `manual-for-ai.md` in full.
This manual is the single source of truth for SSOT authoring. Do not reference other projects' specs.

## 2. Write SSOTs

Create 9 SSOTs in `specs/<project>/`:

| Order | SSOT | Path | Notes |
|---|---|---|---|
| 1 | fullend.yaml | `fullend.yaml` | Project metadata, claims config |
| 2 | SQL DDL | `db/*.sql` | Tables, FK, indexes |
| 3 | sqlc queries | `db/queries/*.sql` | `-- name: Method :cardinality` |
| 4 | OpenAPI | `api/openapi.yaml` | operationId, x- extensions, securitySchemes |
| 5 | SSaC | `service/**/*.ssac` | 11 sequence types + @subscribe trigger, operationId match, .ssac extension |
| 6 | Model | `model/*.go` | @dto types (CurrentUser is auto-generated) |
| 7 | Mermaid stateDiagram | `states/*.md` | State transitions, event = operationId |
| 8 | OPA Rego | `policy/*.rego` | @ownership, allow rules |
| 9 | Gherkin Scenario | `scenario/*.feature` | @scenario, @invariant |
| 10 | STML | `frontend/*.html` | data-fetch, data-action, data-bind |
| Optional | Func Spec | `func/<pkg>/*.go` | @func, Request/Response struct. Import path: `internal/<pkg>` (specs에서 artifacts로 복사됨) |

### fullend.yaml Optional Config

| Config | Purpose | Default |
|---|---|---|
| `session.backend` | Session storage | (none) |
| `cache.backend` | Cache storage | (none) |
| `file.backend` | File storage | (none) |
| `queue.backend` | Queue pub/sub | (none) |
| `authz.package` | Custom authz package (skip default codegen) | (none) |

### Authoring Principles

- operationId is the key that connects all SSOTs. Names must match exactly across OpenAPI, SSaC, STML, States, and Scenario.
- DDL table names: snake_case plural. SSaC Model names: PascalCase singular (`gigs` ↔ `Gig`).
- stateDiagram transition event = SSaC function name = OpenAPI operationId.
- OPA @ownership tables and columns must exist in DDL.
- Gherkin step operationId, METHOD, and JSON fields must match OpenAPI.
- x-sort/x-filter allowed columns must exist in DDL, preferably indexed.

## 3. Generate External Models (Optional)

If the project consumes external APIs, generate Go models from their OpenAPI docs:

```bash
fullend gen-model <openapi-source> <output-dir>
fullend gen-model https://api.stripe.com/openapi.yaml specs/<project>/external/
```

This generates a `.go` file with interface + types + HTTP client. Place it wherever the project's SSaC imports reference.

## 4. Validate

```bash
cd ~/.clari/repos/fullend
go build ./cmd/fullend/
./fullend validate specs/<project>
```

- Fix all ERRORs before proceeding to codegen.
- Review WARNINGs — fix if unintended.
- Do not run gen until validation passes.

## 5. Generate

```bash
./fullend gen specs/<project> artifacts/<project>
```

Output:
- `artifacts/<project>/backend/` — Go backend (gin)
- `artifacts/<project>/frontend/` — React frontend
- `artifacts/<project>/tests/` — Hurl tests (smoke + scenario + invariant)

## 6. Build Backend

```bash
cd artifacts/<project>/backend
go build -o server ./cmd/
```

If build fails, suspect SSOT or codegen bug. Never edit generated code directly.

## 7. DB Setup + Server Start

```bash
# Apply DDL (table order: respect FK dependencies)
for f in <tables in dependency order>; do
  psql -h localhost -p <port> -U postgres -d <dbname> -f specs/<project>/db/$f.sql
done

# Start dummy SMTP for @call mail.* (accepts all, discards)
python3 ~/.clari/repos/fullend/artifacts/scripts/dummy-smtp.py &

# Start server (DISABLE flags for smoke testing)
DISABLE_AUTHZ=1 DISABLE_STATE_CHECK=1 JWT_SECRET=test-secret-key \
  OPA_POLICY_PATH=../../specs/<project>/policy/authz.rego \
  SMTP_HOST=127.0.0.1 SMTP_PORT=2525 SMTP_USERNAME=test SMTP_PASSWORD=test SMTP_FROM=test@test.com \
  ./server -dsn "postgres://..." &
```

## 8. Run Hurl Tests

```bash
cd artifacts/<project>
hurl --test --variable host=http://localhost:8080 tests/*.hurl
```

Pass criteria:
- `smoke.hurl` — OpenAPI endpoint smoke tests
- `scenario-*.hurl` — Business scenario tests
- `invariant-*.hurl` — Invariant verification tests

## Error Handling

| Stage | On Failure |
|---|---|
| validate | Fix SSOTs → re-validate |
| gen | Codegen bug → report immediately, no workarounds |
| go build (authz) | `@auth` generates `authz.Check()` via `fullend/pkg/authz`. Set `OPA_POLICY_PATH` to `.rego` file path. Set `DISABLE_AUTHZ=1` to bypass. |
| go build (config) | `config.*` is NOT supported in SSaC inputs. Func reads its own config via `os.Getenv()`. |
| go build | SSOT or codegen bug → never edit generated code |
| hurl --test | Classify cause (SSOT vs codegen) → report |

Never edit generated code (`artifacts/`). The root cause is always in SSOTs or codegen.
