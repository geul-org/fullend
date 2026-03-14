# Mutation Test — States 단독

### MUT-STATES-001: 상태명 대소문자 (내부 일관성)
- 대상: `specs/gigbridge/states/gig.md`
- 변경: `draft --> open : PublishGig` → `Draft --> open : PublishGig`
- 기대: ERROR — 동일 상태명의 대소문자 불일치 (draft vs Draft)
- 결과: PASS — Phase025: States 파서 단계에서 case-insensitive 중복 검출
