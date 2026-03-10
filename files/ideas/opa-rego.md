1. `specs/policy/*.rego`에 인가 정책 선언
2. SSaC `authorize`가 Rego에 질의하도록 코드젠 수정
3. `fullend validate`에 Rego ↔ SSaC 교차 검증 추가 — authorize 있는데 policy 없거나 그 역
4. `fullend validate`에 Rego ↔ OpenAPI 교차 검증 추가 — policy에 있는 operationId가 OpenAPI에 존재하는가
5. SSOT 6개로 확장, CLI 리포트에 Policy 행 추가