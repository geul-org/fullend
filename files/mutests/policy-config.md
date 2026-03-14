# Mutation Test — Policy ↔ Config

### MUT-POLICY-CONFIG-001: Policy role명 변경
- 대상: `specs/gigbridge/policy/authz.rego`
- 변경: `input.role == "client"` → `input.role == "Client"` (PublishGig 규칙)
- 기대: ERROR — fullend.yaml roles에 "Client" 없음
- 결과: PASS — Phase013 구현 후 검출 성공
