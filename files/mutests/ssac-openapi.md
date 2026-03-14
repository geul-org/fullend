# Mutation Test — SSaC ↔ OpenAPI

### MUT-SSAC-OPENAPI-001: OpenAPI operationId 대소문자
- 대상: `specs/gigbridge/api/openapi.yaml`
- 변경: `operationId: Login` → `operationId: login`
- 기대: ERROR — SSaC function "Login"과 불일치
- 결과: PASS — SSaC→OpenAPI + OpenAPI→SSaC 양방향 검출

### MUT-SSAC-OPENAPI-002: OpenAPI 응답 property 대소문자
- 대상: `specs/gigbridge/api/openapi.yaml`
- 변경: Login 응답 `AccessToken` → `accessToken`
- 기대: ERROR — json tag `access_token`과 불일치 (둘 다 틀림)
- 결과: PASS — Phase012 shorthand 검증이 access_token vs accessToken 잡음

### MUT-SSAC-OPENAPI-003: SSaC 함수명 변경
- 대상: `specs/gigbridge/service/gig/create_gig.ssac`
- 변경: `func CreateGig()` → `func Creategig()`
- 기대: ERROR — OpenAPI operationId "CreateGig"과 불일치
- 결과: PASS — SSaC→OpenAPI + OpenAPI→SSaC 검출

### MUT-SSAC-OPENAPI-004: @response 필드명 변경
- 대상: `specs/gigbridge/service/gig/create_gig.ssac`
- 변경: `@response { gig: gig }` → `@response { Gig: gig }`
- 기대: ERROR — OpenAPI 응답 property "gig"과 불일치
- 결과: PASS — @response→OpenAPI + OpenAPI→@response 양방향

### MUT-SSAC-OPENAPI-005: OpenAPI 에러 응답 삭제
- 대상: `specs/gigbridge/api/openapi.yaml`
- 변경: GetGig의 `404` 응답 삭제
- 기대: ERROR — SSaC @empty 기본 404에 대응하는 OpenAPI 응답 없음
- 결과: PASS — @empty→OpenAPI 404 누락 정확히 검출
