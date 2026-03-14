# Mutation Test Phase 2 — crosscheck 심화 검출력 검증

## 목적

Phase 1에서 주요 경로를 커버했다. Phase 2는 아직 안 건드린 crosscheck 규칙을 검증한다.

## 방법

1. 변경 적용
2. `go run ./cmd/fullend validate specs/gigbridge` 실행
3. 검출 여부 기록 (PASS = 잡음, FAIL = 못 잡음)
4. `git checkout -- specs/gigbridge/` 로 되돌림

## 시나리오 (개별 항목은 각 crosscheck 파일로 이동)

| 구 ID | 신 ID | 파일 |
|---|---|---|
| MUT-16 | MUT-CONFIG-POLICY-001 | config-policy.md |
| MUT-17 | MUT-CONFIG-OPENAPI-001 | config-openapi.md |
| MUT-18 | MUT-SSAC-OPENAPI-005 | ssac-openapi.md |
| MUT-19 | MUT-POLICY-DDL-001 | policy-ddl.md |
| MUT-20 | MUT-DDL-SSAC-001 | ddl-ssac.md |
| MUT-21 | MUT-OPENAPI-DDL-003 | openapi-ddl.md |
| MUT-22 | MUT-DDL-001 | ddl.md |
| MUT-23 | MUT-SCENARIO-OPENAPI-002 | scenario-openapi.md |

**결과: 8/8 검출 (100%)**
