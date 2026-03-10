# fullend TODO

## 미완료 항목

### 1. scenario hurl 테스트 실행 검증
- `scenario-*.hurl`, `invariant-*.hurl` 파일들을 실제 서버에서 돌려보지 않음
- scenario-gen이 생성한 hurl이 올바르게 동작하는지 확인 필요

### ~~2. CanTransition 시그니처 통일~~ ✅ Phase 008 완료

### 3. pkg/middleware와 생성 미들웨어 역할 정리 → Phase 009 계획 완료
- Phase 009에서 `pkg/middleware/bearerauth.go` 삭제 + fallback 제거 예정

### 4. 다른 프로젝트로 일반화 검증
- dummy-lesson 외에 dummy-study 등 다른 프로젝트로 validate → gen → build → hurl 전체 사이클 검증
- 코너 케이스 발견 및 수정
