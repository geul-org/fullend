# Mutation Test — DDL 단독

### MUT-DDL-001: Sensitive 컬럼명 변경 (내부 sensitive 패턴)
- 대상: `specs/gigbridge/db/users.sql`
- 변경: `password_hash` → `pw_hash`
- 기대: WARNING — sensitive 패턴 매칭
- 결과: PASS — pw_hash는 "hash" 서브스트링 패턴으로 검출. Phase014: sensitive 패턴 20개로 확장
