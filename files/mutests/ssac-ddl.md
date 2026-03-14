# Mutation Test — SSaC ↔ DDL

### MUT-SSAC-DDL-001: DDL 테이블명 변경
- 대상: `specs/gigbridge/db/gigs.sql`
- 변경: `CREATE TABLE gigs` → `CREATE TABLE gig`
- 기대: ERROR — SSaC @get Gig.List의 테이블 "gigs"와 불일치
- 결과: PASS — SSaC @result↔DDL 9건 + index 누락 등 대량 검출
