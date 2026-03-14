# Mutation Test — OpenAPI ↔ DDL

### MUT-OPENAPI-DDL-001: DDL 컬럼명 언더스코어
- 대상: `specs/gigbridge/db/gigs.sql`
- 변경: `client_id` → `clientid` (언더스코어 제거)
- 기대: ERROR — OpenAPI x-model Gig의 `client_id`와 불일치
- 결과: PASS — x-include FK + Policy ownership 컬럼 부재 검출

### MUT-OPENAPI-DDL-002: DDL 컬럼 추가 누락
- 대상: `specs/gigbridge/api/openapi.yaml`
- 변경: Gig schema에 `budget` property 제거
- 기대: WARNING — DDL gigs.budget이 OpenAPI에 없음
- 결과: PASS — SSaC @post budget 필드 OpenAPI 부재 검출

### MUT-OPENAPI-DDL-003: OpenAPI 유령 property
- 대상: `specs/gigbridge/api/openapi.yaml`
- 변경: Gig schema에 `rating: { type: integer }` 추가
- 기대: WARNING — DDL gigs에 rating 컬럼 없음
- 결과: PASS — Phase014: checkGhostProperties 추가로 검출 (ERROR)
