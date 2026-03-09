# TODO: 실행 가능한 서버 산출을 위한 미해결 과제

## 타입 통합 (최우선)

현재 같은 모델이 3벌 다른 타입으로 생성됨:

| 생성기 | 타입 | 특징 |
|---|---|---|
| oapi-codegen | `api.Course` | JSON 태그, 포인터 필드 |
| ssac model | `model.CourseModel` interface | 비즈니스 메서드 시그니처 |
| sqlc | `db.Course` | DB 컬럼 매핑, `context.Context` |

**결정 필요:** 어떤 타입을 기준으로 통일할 것인가? 또는 변환 레이어를 자동 생성할 것인가?

## sqlc 시그니처 불일치

```go
// ssac model interface (현재)
FindByID(courseID string) (*Course, error)

// sqlc가 실제 생성하는 것
FindByID(ctx context.Context, id int64) (Course, error)
```

차이점:
- `context.Context` 유무
- 포인터 vs 값 반환
- 파라미터 이름/타입 (`courseID string` vs `id int64`)

## sqlc.yaml 부재

dummy 프로젝트에 sqlc.yaml 설정 파일이 없음. sqlc가 쿼리 구현을 생성하려면 필요.

## 프론트엔드 런타임

stml이 생성한 TSX가 의존하는:
- React hooks (useQuery, useMutation 등)
- API 클라이언트 래퍼
- 라우팅 연결

## 인증/인가

- `currentUser` — JWT 파싱 미들웨어 필요
- `authz.Check` — 권한 검사 구현 필요

## 참고: 기술 스택 목표

```
Backend: Go 1.24, Gin, oapi-codegen v2
Frontend: React 18, TypeScript 5, Vite 5, Tailwind CSS 3
Database: PostgreSQL 15+
Storage: S3 (운영), 로컬 파일시스템 (개발)
Infra: Terraform, AWS (EC2 + CloudFront + S3)
Build: Makefile
```

## Phase 10 계획 상태

`specs/plans/Phase010-GlueCodeGen.md` 작성됨. 위 과제 해결 후 실행.
