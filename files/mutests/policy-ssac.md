# Mutation Test — Policy ↔ SSaC

### MUT-POLICY-SSAC-001: Policy action명 대소문자
- 대상: `specs/gigbridge/policy/authz.rego`
- 변경: `input.action == "PublishGig"` → `input.action == "publishGig"`
- 기대: ERROR — SSaC @auth action "PublishGig"과 불일치
- 결과: PASS — Policy↔SSaC 양방향 검출

### MUT-POLICY-SSAC-002: Policy resource명 오타
- 대상: `specs/gigbridge/policy/authz.rego`
- 변경: `input.resource == "gig"` → `input.resource == "gigs"`
- 기대: ERROR — SSaC @auth resource "gig"과 불일치
- 결과: PASS — Policy↔SSaC resource 불일치 검출
