# Mutation Test — SSaC → Func

## CheckFuncs (SSaC @call → Func Spec)

### MUT-SSAC-FUNC-001: SSaC @call 패키지명 오타
- 대상: `specs/gigbridge/service/auth/login.ssac`
- 변경: `auth.IssueToken` → `Auth.IssueToken`
- 기대: ERROR — funcspec package "auth"와 불일치
- 결과: PASS — Func↔SSaC 패키지 불일치 검출

## CheckAuthz (SSaC @auth → pkg/authz CheckRequest)
