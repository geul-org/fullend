# Fullend Validate 검증 현황

## SSOT 소비 관계

```
DDL ←── OpenAPI ←── Config (middleware → securitySchemes)
 ↑         ↑
 ├── SSaC ─┘←── Policy (action/resource → @auth)
 │    ↑              ↑
 │    ├── States     ├── DDL (@ownership)
 │    ├── Func       ├── Config (roles, claims)
 │    └── Config     └── States
 │
 ├── States (state field → DDL column)
 └── DDL → SSaC (coverage)

Scenario → OpenAPI
STML → OpenAPI (검증 미구현)
```

### 단방향

| 소비자 | 대상 | Check 함수 | 검증 내용 |
|---|---|---|---|
| OpenAPI | DDL | `CheckOpenAPIDDL` | property↔column, x-include/x-sort/x-filter↔FK/index, ghost/missing property |
| SSaC | DDL | `CheckSSaCDDL` | @result type↔DDL table, param type↔DDL column |
| SSaC | Func | `CheckFuncs` | @call pkg.Function 존재, 시그니처 일치 |
| SSaC | Func | `CheckAuthz` | @auth inputs↔pkg/authz CheckRequest fields |
| SSaC | Config | `CheckClaims` | currentUser.X↔claims 정의 |
| Policy | SSaC | `CheckPolicy` | Rego action/resource↔SSaC @auth |
| Policy | DDL | `CheckPolicy` | @ownership table.column↔DDL column |
| Policy | Config | `CheckClaimsRego` | Rego input.claims.X↔claims 정의 |
| Policy | Config | `CheckRoles` | Rego input.role↔roles 목록 |
| Policy | States | `CheckPolicy` | Rego 상태 참조↔States 정의 |
| Config | OpenAPI | `CheckMiddleware` | middleware↔securitySchemes |
| Scenario | OpenAPI | `CheckHurlFiles` | Hurl path/method↔OpenAPI endpoint |

### 양방향

| 쌍 | Check 함수 | A→B | B→A |
|---|---|---|---|
| SSaC ↔ OpenAPI | `CheckSSaCOpenAPI` | func name→operationId, @response→response schema, @empty/@exists→error response | operationId→SSaC func 존재 |
| SSaC ↔ States | `CheckStates` | @state diagramID→diagram 존재, func name→유효 전이 | transition event→SSaC func 존재, event without @state→WARNING |
| SSaC ↔ DDL | `CheckDDLCoverage` | (CheckSSaCDDL에서 커버) | DDL table→SSaC에서 사용됨 (coverage) |
| States ↔ DDL | `CheckStates` | @state input field→DDL column | (역방향 없음) |

### 단독 검증

| SSOT | Check 함수 | 검증 내용 |
|---|---|---|
| DDL | `CheckSensitiveColumns` | 칼럼명 sensitive 패턴 매칭 + @sensitive 어노테이션 대조 |
| SSaC | `CheckQueue` | @publish topic↔@subscribe topic 내부 일관성 |
| States | `statemachine.Parse` | 상태명 case-insensitive 중복 검출 |

---

## 개선안

### 제거 후보 (1건)

**States → OpenAPI** (`CheckStates` #4: transition event → operationId)

전이적 중복이다:
- States → SSaC (`CheckStates` #1): event → SSaC func 존재
- SSaC → OpenAPI (`CheckSSaCOpenAPI` Rule 3): func name → operationId 존재

두 검증의 합성으로 States event → operationId는 보장된다. 독자적으로 잡아내는 에러 없음.

제거 시 효과: States event에 SSaC func이 없을 때 ERROR 1건(States→SSaC)만 출력. 현재는 2건(+States→OpenAPI) 중복 출력.

### 추가 후보 (2건)

**1. Func → SSaC 커버리지 (`CheckFuncCoverage`)**

현황: DDL → SSaC 커버리지는 `CheckDDLCoverage`로 구현됨. Func은 없음.

문제: 프로젝트 `func/` 디렉토리에 함수 스펙을 작성했지만 아무 SSaC `@call`도 참조하지 않으면 죽은 코드. 현재 미검출.

제안: WARNING 수준. `func/` 파서 결과와 SSaC @call 참조 목록 대조.

**2. STML → OpenAPI 크로스체크 (`CheckSTMLOpenAPI`)**

현황: STML 파서·validator는 있으나, STML이 참조하는 API endpoint의 OpenAPI 존재 여부를 검증하는 crosscheck 없음.

문제: STML 페이지가 `POST /api/gigs`를 호출하는데 OpenAPI에 해당 path/method가 없으면 현재 미검출.

제안: ERROR 수준. STML API call의 path/method → OpenAPI endpoint 존재 확인.
