# ZenFlow try05 Report

## 소요시간
- 시작: 2026-03-15 12:47:00
- 종료: 2026-03-15 12:53:52
- 소요: 약 7분

## 결과
- fullend validate: PASS (3 warnings)
- fullend gen: PASS
- go build: PASS
- hurl scenario-happy-path: PASS (7 requests)
- hurl invariant-tenant-breach: PASS (4 requests)
- hurl invariant-insufficient-credits: PASS (3 requests)
- hurl smoke: PASS (9 requests)

## SSOT 구성
- fullend.yaml: JWT claims (ID, Email, Role, OrgID)
- OpenAPI: 11 endpoints (register, login, CRUD workflows, actions, execute, logs)
- DDL: 5 tables (organizations, users, workflows, actions, execution_logs)
- SSaC: 11 service functions
- Model: 1 file (package declaration)
- STML: 3 pages (login, workflows, workflow-detail)
- States: 1 diagram (workflow), 5 transitions
- Policy: 1 rego file, 8 allow rules, 1 ownership mapping
- Scenario: 3 hurl files
- Func: 2 custom funcs (billing.checkCredits, worker.processActions)

## 설계 결정
1. **UUID → BIGSERIAL**: fullend 코드젠과 호환을 위해 int64 기반 ID 사용
2. **Org isolation via query**: Rego authz의 CheckRequest가 UserID/Role/ResourceID만 지원하므로, 조직 격리는 sqlc 쿼리에서 `WHERE org_id = $2` 필터로 구현
3. **Credits check via pure func**: DB 접근 불가한 func 제약으로, Organization을 @get으로 조회 후 billing.CheckCredits에 Balance 값 전달
4. **Numeric literal 우회**: SSaC가 숫자 리터럴(100, 1)을 변수로 인식하는 이슈 → DDL DEFAULT 값으로 대체
5. **State self-transition**: ExecuteWorkflow는 상태 변경 없이 active 상태 확인만 필요 → `active --> active: ExecuteWorkflow` 자기 전이로 해결

## Warnings (미해결)
- Claims ↔ Rego: user_id, email, org_id claims가 Rego에서 미참조 (org isolation을 쿼리 레벨에서 처리하므로 불가피)

## 발견된 fullend 이슈
- SSaC 파서가 숫자 리터럴(100, 1 등)을 변수명으로 인식하여 "변수가 선언되지 않았습니다" 에러 발생. 매뉴얼에는 "Numeric: 1, 42, 3.14, -1" 지원으로 명시됨. DDL DEFAULT로 우회 가능하나 파서 버그로 판단.
