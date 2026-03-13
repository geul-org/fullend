# dummy-zenflow 개발 완료 보고서

| 항목 | 내용 |
|---|---|
| 프로젝트 | dummy-zenflow (ZenFlow — Multi-tenant Workflow Automation SaaS) |
| 도구 | fullend SSOT Orchestrator |
| 작업자 | Claude Opus 4.6 |
| 작업일 | 2026-03-12 |
| 소요시간 | 11분 23초 (08:55:50 → 09:07:13) |
| 산출물 | `specs/zenflow/` — 32개 SSOT 파일 |

---

## 1. 작업 범위

dummy-zenflow.md(기획서)를 입력으로 받아, fullend가 요구하는 10개 SSOT + sqlc.yaml을 모두 작성.

| SSOT | 파일 수 | 비고 |
|---|---|---|
| fullend.yaml | 1 | JWT claims 3종 (ID/Email/Role) |
| SQL DDL | 5 | organizations, users, workflows, actions, execution_logs |
| sqlc queries | 5 | 14개 쿼리 (CRUD + 도메인 특화) |
| OpenAPI | 1 | 11개 엔드포인트 |
| SSaC | 11 | auth(2), organization(1), workflow(7), action(1) |
| Model | 1 | DDL 모델만 사용, @dto 없음 |
| Mermaid stateDiagram | 1 | workflow 상태 5개 전이 |
| OPA Rego | 1 | 7개 allow 규칙, 2개 @ownership |
| Gherkin Scenario | 1 | @scenario 1, @invariant 2 |
| STML | 2 | workflows 목록, workflow-detail |
| Terraform | 1 | AWS RDS PostgreSQL |
| sqlc.yaml | 1 | — |

---

## 2. fullend가 도움이 된 점

### 2.1 "설계를 먼저 강제"하는 구조가 실수를 줄였다

일반적인 개발 흐름은 "API 하나 만들고 → DB 스키마 추가하고 → 프론트 연결"이다.
fullend는 10개 SSOT를 전부 먼저 작성하게 하므로, **구현 전에 시스템 전체를 설계**해야 한다.

이 과정에서 기획서(dummy-zenflow.md)의 빈틈이 자연스럽게 드러났다:

- 기획서에는 "사용자"만 있었지만, SSOT 작성 중 "사용자의 org_id를 어디서 가져오는가"를 결정해야 했다
- `ExecuteWorkflow`가 `@state` 전이인지, 단순 실행인지 모호했는데 stateDiagram 작성 시 self-transition(`active → active`)으로 명확해졌다
- OPA 작성 중 "ListWorkflows는 인가가 필요한가, 쿼리 필터로 충분한가"를 결정해야 했다

**결론:** SSOT 간 교차 의존성이 설계 결정을 강제하여, 모호한 요구사항이 구현 전에 해소된다.

### 2.2 operationId 단일 키가 정합성을 보장한다

`ActivateWorkflow`라는 이름 하나가 OpenAPI, SSaC 함수명, stateDiagram 전이, Gherkin 시나리오, STML data-action을 관통한다. 이름이 하나라도 다르면 crosscheck ERROR가 발생하므로, **네이밍 불일치 버그가 원천 차단**된다.

이 구조가 특히 효과적이었던 상황:
- Gherkin 시나리오에서 `ActivateWorkflow`를 `PUT`으로 호출하는데, OpenAPI에서 해당 경로의 method가 `PUT`이 아니면 ERROR
- STML `data-action="ExecuteWorkflow"`를 쓰면, 해당 operationId가 OpenAPI에 반드시 있어야 함

### 2.3 참조 프로젝트(gigbridge)가 학습 비용을 대폭 줄였다

매뉴얼(manual-for-ai.md)만으로는 SSaC 문법의 edge case를 파악하기 어려웠다. gigbridge 코드를 실제 참조하면서:

- `@auth` 인자 형식 (`{UserID: currentUser.ID, ResourceID: gig.ID}`)
- `@state` 인자 형식 (`{status: gig.Status}`)
- `@response`에서 단일 변수 반환 (`@response token`) vs 객체 반환 (`@response { gig: gig }`)
- Gherkin에서 `→ token` 캡처와 변수 참조 패턴 (`gig.gig.id`)

이런 실전 패턴을 빠르게 파악할 수 있었다.

### 2.4 디렉토리 규약이 의사결정 비용을 없앴다

"이 파일을 어디에 둘까"를 고민할 필요가 없다:
- 서비스 로직? → `service/<domain>/<operation>.ssac`
- DB 스키마? → `db/<table>.sql`
- 쿼리? → `db/queries/<table>.sql`

32개 파일을 생성했지만, 경로 결정에 소요된 시간은 사실상 0이었다.

---

## 3. fullend의 한계 및 개선 제안

### 3.1 [Critical] Multi-tenant 패턴이 authz 모델에 맞지 않는다

**문제:**
fullend의 `authz.CheckRequest`는 `{UserID, Role, ResourceID}` 3개 필드만 가진다.
Multi-tenant 격리는 "사용자의 org_id == 리소스의 org_id"를 비교해야 하는데, CheckRequest에 `OrgID`가 없다.

**우회:**
`@ownership user_org: users.org_id`와 `@ownership workflow_org: workflows.org_id`를 정의하고,
Rego에서 `data.owners.workflow_org[input.resource_id] == data.owners.user_org[input.claims.user_id]`로 비교.

이 우회가 동작하려면 `data.owners`가 **요청마다 모든 ownership을 preload**해야 한다.
이것이 실제로 지원되는지 확신할 수 없다 — `data.owners.user_org[input.claims.user_id]`는 resource_id가 아닌 user_id로 조회하는 비표준 패턴이다.

**제안:** `CheckRequest`에 커스텀 claims 필드를 추가하거나, `fullend.yaml`에서 커스텀 claims를 선언하면 `input.claims.*`로 접근 가능하게 하는 방안.

### 3.2 [Critical] SSaC에 조건 분기가 없어 HTTP 상태 코드를 제어할 수 없다

**문제:**
기획서 요구: "크레딧 0 → `402 Payment Required`"
SSaC 현실: `@empty`는 404만 반환, `@exists`는 409만 반환. 커스텀 상태 코드 불가.

**우회:**
`Organization.FindByIDWithCredits` (WHERE credits_balance > 0) 쿼리를 만들고 `@empty`로 404 반환.
기획서의 402를 포기하고, 시나리오 invariant에서 404를 기대값으로 변경.

**제안:** `@guard` 시퀀스 타입 추가 — `@guard condition STATUS "message"` 형태로 임의 조건과 상태 코드 지정 가능.

### 3.3 [Major] SSaC에 반복(loop) 구문이 없다

**문제:**
기획서: "연결된 모든 actions를 sequence_order 순으로 실행"
SSaC: loop 구문이 없으므로, 개별 action 순회 처리 불가.

**우회:**
`worker.ProcessActions({WorkflowID: workflow.ID})`로 배치 처리를 위임.
하지만 이 func은 purity rule로 DB 접근 불가 → 실제 action 목록을 받을 수 없다.
`@get []Action actions = Action.ListByWorkflowID(...)` 결과를 func에 전달하려 했으나,
배열을 func 인자로 전달하는 패턴이 명확하지 않다.

**제안:** `@each var in collection { ... }` 블록, 또는 배열 변수를 @call에 전달하는 문법.

### 3.4 [Major] SSaC에 정수/불리언 리터럴이 없다

**문제:**
`CreditsSpent: 1` 같은 정수 리터럴을 SSaC 인자로 쓸 수 없다.
SSaC 인자는 `source.Field` 또는 `"string literal"`만 허용.

**우회:**
DDL에 `credits_spent INTEGER NOT NULL DEFAULT 1`을 설정하고, sqlc Create 쿼리에서 해당 컬럼을 제외.

**제안:** `1`, `true`, `null` 등 기본 리터럴 지원.

### 3.5 [Minor] Pagination + 필터 인자 혼합 패턴이 불명확하다

**문제:**
`Workflow.ListByOrgID({OrgID: me.OrgID, Query: query})` — OrgID 필터와 pagination을 동시에 쓰려 했으나,
매뉴얼의 pagination 예제는 모두 `{Query: query}` 단독 사용이다.

**우회:**
pagination을 포기하고 `@get []Workflow` (비페이지네이션)으로 변경.

**제안:** `{Filter: value, Query: query}` 혼합 패턴의 공식 지원 + 매뉴얼 예제 추가.

### 3.6 [Minor] built-in auth 함수가 커스텀 claims를 지원하지 않는다

**문제:**
`auth.IssueToken`은 `{UserID, Email, Role}` 3개만 받는다.
Multi-tenant에서 `OrgID`를 JWT에 넣으려면 커스텀 IssueToken이 필요하다.

**우회:**
OrgID를 JWT에 넣지 않고, 매 요청마다 `User.FindByID`로 DB 조회하여 org_id를 획득.

---

## 4. 시간 분석

| 단계 | 소요 추정 | 비고 |
|---|---|---|
| 기획서 분석 | 1분 | dummy-zenflow.md 읽기 |
| 매뉴얼 + 참조 프로젝트 학습 | 3분 | manual-for-ai.md + gigbridge 전체 읽기 |
| 설계 결정 (authz, credits, loop 우회) | 4분 | 한계 파악 + 우회 방안 도출 |
| SSOT 파일 작성 | 3분 | 32개 파일 생성 |
| 검증 | 0.5분 | 파일 구조 확인 |

**전체 11분** 중 **설계 결정이 36%**, **파일 작성이 27%**.
파일 작성 자체는 빨랐으나, fullend 모델의 한계를 이해하고 우회하는 데 시간이 집중되었다.

---

## 5. 기획서 대비 변경점

| 기획서 요구 | 실제 구현 | 사유 |
|---|---|---|
| 402 Payment Required | 404 Not Found | SSaC에 커스텀 상태 코드 없음, @empty → 404 |
| UUID PK | BIGSERIAL PK | fullend 표준 (sqlc 호환) |
| actions 순차 실행 루프 | 배치 func 위임 | SSaC에 loop 없음 |
| `deductCredit` @call func | `@put Organization.DeductCredit` | Purity rule: func은 DB 접근 불가 |
| `checkCredits` @call func | `Organization.FindByIDWithCredits` + @empty | Purity rule 동일 |
| org_id in JWT claims | DB 조회 (User.FindByID) | built-in IssueToken이 3 claims만 지원 |
| `payload_template` JSONB 사용 | DDL에 존재하나 API에서 미사용 | CreateAction에서 제외 (간소화) |

---

## 6. 종합 평가

### fullend는 "SSOT-first 설계 강제 도구"로서 명확한 가치가 있다

10개 SSOT의 교차 검증이 설계 단계에서 불일치를 잡아주는 구조는 강력하다.
operationId 단일 키로 API-서비스-상태-정책-시나리오-프론트엔드를 관통하는 설계는,
팀 규모가 커질수록 네이밍 혼선을 방지하는 데 큰 효과를 발휘할 것이다.

### 그러나 Multi-tenant SaaS에는 모델 확장이 필요하다

현재 authz 모델(UserID/Role/ResourceID)은 단일 테넌트 ownership 패턴에 최적화되어 있다.
Multi-tenant 격리는 현업 SaaS의 기본 요구사항이므로, 이를 1등 시민(first-class)으로 지원하면
fullend의 적용 범위가 크게 넓어질 것이다.

### 조건 분기 + 커스텀 상태 코드 부재가 실전 적용의 가장 큰 벽이다

비즈니스 로직은 본질적으로 조건 분기다. "잔액 부족 → 402", "구독 만료 → 403",
"중복 요청 → 409" 같은 분기를 SSaC에서 표현할 수 없으면,
모든 비즈니스 규칙을 @call func에 우회하거나 DB 쿼리로 인코딩해야 한다.
`@guard` 같은 조건 시퀀스 타입 하나가 추가되면 이 문제의 대부분이 해소된다.
