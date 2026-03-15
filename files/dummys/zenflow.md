# ZenFlow — 더미 프로젝트 명세

## 1. Domain Overview

ZenFlow는 기업용(Multi-tenant) 업무 자동화 SaaS입니다. 조직(Organization)은 워크플로우를 설계하고, 각 워크플로우는 여러 액션(Action)을 순차적으로 실행합니다. 모든 실행은 조직의 구독 플랜 및 잔여 크레딧에 종속됩니다.

## 2. Entity & DDL

```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    plan_type TEXT CHECK (plan_type IN ('free', 'pro', 'enterprise')),
    credits_balance INTEGER DEFAULT 0
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id),
    email TEXT UNIQUE NOT NULL,
    role TEXT CHECK (role IN ('admin', 'member'))
);

CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id),
    title TEXT NOT NULL,
    trigger_event TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID REFERENCES workflows(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    payload_template JSONB,
    sequence_order INTEGER NOT NULL
);

CREATE TABLE execution_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID REFERENCES workflows(id),
    org_id UUID REFERENCES organizations(id),
    status TEXT,
    credits_spent INTEGER,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 3. State Machine

```mermaid
stateDiagram-v2
    [*] --> draft
    draft --> active: ActivateWorkflow (if credits > 0)
    active --> paused: PauseWorkflow
    paused --> active: ActivateWorkflow
    active --> archived: ArchiveWorkflow
```

## 4. Authorization Rules

```rego
package authz

# @ownership organization: organizations.id
# @ownership workflow: workflows.org_id
# @ownership user_org: users.org_id

default allow = false

is_same_org {
    input.user.org_id == input.resource.org_id
}

allow {
    input.operation == "CreateWorkflow"
    input.user.role == "admin"
}

allow {
    input.operation == "ListWorkflows"
    is_same_org
}

allow {
    input.operation == "ActivateWorkflow"
    input.user.role == "admin"
    is_same_org
}
```

## 5. API & Business Logic

### POST /workflows/{id}/activate (`ActivateWorkflow`)

1. 조직의 `credits_balance` 확인
2. 0 이하면 `402 Payment Required` 반환
3. 상태를 `active`로 변경

### POST /workflows/{id}/execute (`ExecuteWorkflow`)

1. `@auth`로 조직 격리 확인
2. `@state`로 워크플로우가 `active`인지 확인
3. 연결된 모든 `actions`를 `sequence_order` 순으로 로드
4. `@call worker.processAction` 루프 실행
5. `@call billing.deductCredit` 호출 (성공 시 1크레딧 차감)
6. `execution_logs` 기록

## 6. Custom Functions

- `processAction(actionType, payload)`: 외부 API 호출 시뮬레이션
- `checkCredits(orgID)`: 현재 잔액 반환
- `deductCredit(orgID, amount)`: 원자적 크레딧 차감

## 7. E2E Scenario

- **@scenario** Happy Path: Admin이 워크플로우 생성 → 액션 2개 추가 → 활성화(성공) → 실행 → 로그 생성 및 크레딧 차감 확인
- **@invariant** Tenant Breach: Org A의 사용자가 Org B의 `workflow_id`로 실행 요청 → `403 Forbidden`
- **@invariant** Insufficient Credits: 크레딧이 0인 조직이 활성화 시도 → `402 Payment Required`

## 8. 개발 지침

1. `artifacts/AGENTS.md`, `artifacts/manual-for-ai.md` 참고
2. SSOT specs는 `specs/dummys/zenflow-tryNN/`에 작성 (tryNN은 시도 번호)
3. 시작 시간을 기록하고 소요시간을 측정
4. 더미 메일은 `artifacts/scripts/dummy-smtp.py` 사용, DB는 도커로 실행
5. 다른 프로젝트를 참고하지 말고 매뉴얼만 숙지하고 개발
6. fullend에 오류가 있더라도 수정하지 말고, 멈추지 말고 완수. 버그는 `files/bugs/BUGNNN.md`에 리포트.
7. 완료 후 `files/dummys/reports/zenflow-report-n.md` 작성
8. PTY 쉘에서 `!`에 이스케이프 하므로 비밀번호에 사용금지.
