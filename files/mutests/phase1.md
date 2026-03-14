# Mutation Test Phase 1 — gigbridge crosscheck 검출력 검증

## 목적

gigbridge SSOT에 미묘한 오류를 하나씩 주입하고 `fullend validate`가 검출하는지 확인한다.
변경은 최대한 교묘하게 — 대소문자 한 글자, 언더스코어 추가/제거, 복수형 등.

## 방법

1. 변경 적용
2. `go run ./cmd/fullend validate specs/gigbridge` 실행
3. 검출 여부 기록 (PASS = 잡음, FAIL = 못 잡음)
4. `git checkout -- specs/gigbridge/` 로 되돌림

## 시나리오 (개별 항목은 각 crosscheck 파일로 이동)

| 구 ID | 신 ID | 파일 |
|---|---|---|
| MUT-01 | MUT-SSAC-OPENAPI-001 | ssac-openapi.md |
| MUT-02 | MUT-SSAC-OPENAPI-002 | ssac-openapi.md |
| MUT-03 | MUT-OPENAPI-DDL-001 | openapi-ddl.md |
| MUT-04 | MUT-STATES-001 | states.md |
| MUT-05 | MUT-STATES-SSAC-001 | states-ssac.md |
| MUT-06 | MUT-POLICY-SSAC-001 | policy-ssac.md |
| MUT-07 | MUT-POLICY-SSAC-002 | policy-ssac.md |
| MUT-08 | MUT-SSAC-OPENAPI-003 | ssac-openapi.md |
| MUT-09 | MUT-SSAC-OPENAPI-004 | ssac-openapi.md |
| MUT-10 | MUT-SSAC-DDL-001 | ssac-ddl.md |
| MUT-11 | MUT-OPENAPI-DDL-002 | openapi-ddl.md |
| MUT-12 | MUT-STATES-SSAC-002 | states-ssac.md |
| MUT-13 | MUT-POLICY-CONFIG-001 | policy-config.md |
| MUT-14 | MUT-SCENARIO-OPENAPI-001 | scenario-openapi.md |
| MUT-15 | MUT-SSAC-FUNC-001 | ssac-func.md |

**결과: 15/15 검출 (100%)**
