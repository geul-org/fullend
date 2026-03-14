# Mutation Test — Policy ↔ DDL

### MUT-POLICY-DDL-001: Policy ownership 컬럼 변경
- 대상: `specs/gigbridge/policy/authz.rego`
- 변경: `@ownership gig gigs.client_id` → `@ownership gig gigs.owner_id`
- 기대: ERROR — DDL gigs 테이블에 owner_id 컬럼 부재
- 결과: PASS — 재테스트 PASS (초회 sed 패턴 오류)
