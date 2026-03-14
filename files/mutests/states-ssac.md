# Mutation Test — States ↔ SSaC

### MUT-STATES-SSAC-001: 상태 전이 함수명 오타
- 대상: `specs/gigbridge/states/gig.md`
- 변경: `draft --> open : PublishGig` → `draft --> open : Publishgig`
- 기대: ERROR — SSaC function "PublishGig"과 불일치
- 결과: PASS — States↔SSaC + States↔OpenAPI 3건 검출

### MUT-STATES-SSAC-002: 상태 전이 누락
- 대상: `specs/gigbridge/states/gig.md`
- 변경: `under_review --> disputed : RaiseDispute` 행 삭제
- 기대: ERROR — SSaC RaiseDispute의 @state 전이가 States에 없음
- 결과: PASS — States↔SSaC 전이 누락 검출
