# Mutation Test — DDL ↔ SSaC

### MUT-DDL-SSAC-001: DDL 고아 테이블
- 대상: `specs/gigbridge/db/`
- 변경: `CREATE TABLE audit_logs (id BIGSERIAL PRIMARY KEY, action TEXT);` 파일 추가
- 기대: WARNING — SSaC에서 audit_logs 테이블을 사용하지 않음
- 결과: PASS — DDL→SSaC coverage WARNING 검출
