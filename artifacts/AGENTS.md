# fullend SSOT Agent Instructions

## 1. SSOT 문법 숙지 (최우선)

작업 시작 전 반드시 `artifacts/manual-for-ai.md`를 전체 읽는다.
이 매뉴얼이 SSOT 작성의 유일한 기준이다. 다른 프로젝트의 specs를 참고하지 않는다.

## 2. SSOT 작성

`specs/<project>/` 디렉토리에 10개 SSOT를 작성한다.

| 순서 | SSOT | 경로 | 비고 |
|------|------|------|------|
| 1 | fullend.yaml | `fullend.yaml` | 프로젝트 메타데이터, claims 설정 |
| 2 | SQL DDL | `db/*.sql` | 테이블 정의, FK, 인덱스 |
| 3 | sqlc queries | `db/queries/*.sql` | `-- name: Method :cardinality` |
| 4 | OpenAPI | `api/openapi.yaml` | operationId, x- 확장, securitySchemes |
| 5 | SSaC | `service/*.go` | 10개 시퀀스 타입, operationId 일치 |
| 6 | Model | `model/*.go` | @dto 타입 (CurrentUser는 자동 생성) |
| 7 | Mermaid stateDiagram | `states/*.md` | 상태 전이, 이벤트 = operationId |
| 8 | OPA Rego | `policy/*.rego` | @ownership, allow 규칙 |
| 9 | Gherkin Scenario | `scenario/*.feature` | @scenario, @invariant |
| 10 | STML | `frontend/*.html` | data-fetch, data-action, data-bind |
| 11 | Terraform | `terraform/*.tf` | HCL 인프라 선언 |
| 선택 | Func Spec | `func/<pkg>/*.go` | @func, Request/Response struct |

### 작성 원칙

- operationId가 모든 SSOT를 연결하는 핵심 키다. OpenAPI, SSaC, STML, States, Scenario 간 이름이 정확히 일치해야 한다.
- DDL 테이블명은 snake_case 복수형, SSaC Model명은 PascalCase 단수형 (`gigs` <-> `Gig`).
- stateDiagram transition 이벤트명 = SSaC 함수명 = OpenAPI operationId.
- OPA @ownership 테이블·컬럼은 DDL에 실제 존재해야 한다.
- Gherkin step의 operationId, METHOD, JSON 필드는 OpenAPI와 일치해야 한다.
- x-sort/x-filter allowed 컬럼은 DDL에 존재하고, 가능하면 인덱스가 있어야 한다.

## 3. 검증 — fullend validate

```bash
cd ~/.clari/repos/fullend
go build ./cmd/fullend/
./fullend validate specs/<project>
```

- ERROR가 0이 될 때까지 SSOT를 수정한다.
- WARNING은 의도된 것인지 확인하고, 불필요하면 수정한다.
- 검증 통과 전에 코드젠으로 넘어가지 않는다.

## 4. 코드젠 — fullend gen

```bash
./fullend gen specs/<project> artifacts/<project>
```

생성 결과:
- `artifacts/<project>/backend/` — Go 백엔드 (gin)
- `artifacts/<project>/frontend/` — React 프론트엔드
- `artifacts/<project>/tests/` — Hurl 테스트 (smoke + scenario + invariant)

## 5. 백엔드 빌드

```bash
cd artifacts/<project>/backend
go build -o server ./cmd/
```

빌드 실패 시 SSOT 또는 fullend 코드젠 버그를 의심한다. 생성된 코드를 직접 수정하지 않는다.

## 6. DB 준비 + 서버 기동

```bash
# DDL 적용 (테이블 순서: FK 의존성 고려)
for f in <tables in dependency order>; do
  psql -h localhost -p <port> -U postgres -d <dbname> -f specs/<project>/db/$f.sql
done

# 서버 기동 (DISABLE_AUTHZ=1, DISABLE_STATE_CHECK=1은 스모크 테스트용)
DISABLE_AUTHZ=1 DISABLE_STATE_CHECK=1 JWT_SECRET=test-secret-key ./server -dsn "postgres://..." &
```

## 7. Hurl 테스트 실행

```bash
cd artifacts/<project>
hurl --test --variable host=http://localhost:8080 tests/*.hurl
```

전체 통과 기준:
- `smoke.hurl` — OpenAPI 엔드포인트 스모크 테스트
- `scenario-*.hurl` — 비즈니스 시나리오 테스트
- `invariant-*.hurl` — 불변 조건 검증 테스트

## 오류 대응

| 단계 | 실패 시 |
|------|---------|
| validate | SSOT 수정 → 재검증 |
| gen | fullend 코드젠 버그 → 즉시 보고, 우회 금지 |
| go build | SSOT 또는 코드젠 버그 → 생성 코드 직접 수정 금지 |
| hurl --test | SSOT 오류 또는 코드젠 버그 → 원인 분류 후 보고 |

생성된 코드(`artifacts/`)를 직접 수정하는 것은 금지다. 문제의 원인은 항상 SSOT 또는 코드젠에 있다.
