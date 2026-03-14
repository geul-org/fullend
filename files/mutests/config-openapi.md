# Mutation Test — Config ↔ OpenAPI

### MUT-CONFIG-OPENAPI-001: Middleware 제거
- 대상: `specs/gigbridge/fullend.yaml`
- 변경: `middleware: [bearerAuth]` → `middleware: []`
- 기대: ERROR — OpenAPI securitySchemes bearerAuth 존재하지만 미들웨어 미선언
- 결과: PASS — Phase014: middleware nil 조건 제거로 검출
