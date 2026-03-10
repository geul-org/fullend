1. `specs/scenario/*.feature`에 Gherkin으로 시나리오 + 불변식 선언
2. 우리 제약: `POST/GET/PUT/DELETE operationId { JSON } → result` 고정 패턴
3. `@scenario` / `@invariant` 태그로 성격 구분
4. fullend validate가 operationId, request 필드, response 필드를 OpenAPI 교차 체크
5. fullend gen이 `.feature`에서 Hurl 테스트 자동 파생
6. 상태 전이 시나리오는 stateDiagram의 전이와 교차 검증