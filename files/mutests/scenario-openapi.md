# Mutation Test — Scenario ↔ OpenAPI

### MUT-SCENARIO-OPENAPI-001: OpenAPI path 변경
- 대상: `specs/gigbridge/api/openapi.yaml`
- 변경: `/auth/login` → `/auth/Login`
- 기대: ERROR — Hurl scenario의 path와 불일치
- 결과: PASS — Hurl→OpenAPI 6건 path 불일치 검출

### MUT-SCENARIO-OPENAPI-002: Hurl HTTP method 변경
- 대상: `specs/gigbridge/tests/scenario-gig-lifecycle.hurl`
- 변경: `POST {{host}}/auth/login` → `GET {{host}}/auth/login`
- 기대: ERROR — OpenAPI /auth/login에 GET 메서드 미정의
- 결과: PASS — Hurl method↔OpenAPI method 불일치 검출
