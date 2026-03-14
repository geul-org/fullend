# Mutation Test — DDL 단독

### MUT-DDL-001: Sensitive 컬럼명 변경 (@sensitive 제거)
- 대상: `specs/gigbridge/db/users.sql`
- 변경: `password_hash VARCHAR(255) NOT NULL, -- @sensitive` → `pw_hash VARCHAR(255) NOT NULL,` (@sensitive 어노테이션 제거)
- 기대: WARNING — pw_hash가 "hash" 서브스트링 패턴에 매칭, @sensitive 없으므로 경고
- 결과: PASS — Phase014: sensitive 패턴 20개로 확장, @sensitive 미부착 시 검출

### MUT-DDL-002: Sensitive 컬럼명 변경 (@sensitive 유지)
- 대상: `specs/gigbridge/db/users.sql`
- 변경: `password_hash` → `pw_hash` (@sensitive 어노테이션 유지)
- 기대: 무경고 — @sensitive가 붙어있으므로 이미 인지된 것으로 간주, 패턴 매칭 스킵
- 결과: PASS — @sensitive 어노테이션이 있으면 경고하지 않는 정상 동작 확인
