# Mutation Test — Config ↔ Policy

### MUT-CONFIG-POLICY-001: Claims 필드명 변경
- 대상: `specs/gigbridge/fullend.yaml`
- 변경: `claims.ID: user_id` → `claims.ID: userId`
- 기대: ERROR — Rego input.claims 참조와 fullend.yaml claims 불일치
- 결과: PASS — Phase014: CheckClaimsRego 추가로 검출
