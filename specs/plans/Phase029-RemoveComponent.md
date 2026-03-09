✅ 완료

# Phase 029: @component 폐지 — @func만 허용

## 목표

`@sequence call`에서 `@component` 지원을 완전 제거하고 `@func`만 허용한다.

## 배경

- `@component`는 interface만 선언하고 구현체 주입이 codegen에서 보장되지 않아 런타임 nil panic 발생
- notification 같은 복잡 기능은 `@func` 하나로도 해결 안 됨 — SSOT 범위 밖
- `@func pkg.funcName` 패턴으로 외부 로직 호출을 통일

ssac 측 수정은 `수정지시서014`로 별도 요청 완료. fullend 측 변경만 다룬다.

## 변경 항목

### A. dummy-lesson 스펙 정리

| 파일 | 변경 |
|------|------|
| `specs/dummy-lesson/service/enrollment/enroll_course.go` | `@sequence call` + `@component notification` 시퀀스 삭제 |
| `specs/dummy-lesson/model/notification.go` | 파일 삭제 |

### B. crosscheck — @component ERROR 추가

| 파일 | 변경 |
|------|------|
| `internal/crosscheck/crosscheck.go` | SSaC 파싱 결과에서 `@component` 사용 감지 시 ERROR 발생: `"@component는 폐지되었습니다. @func pkg.funcName을 사용하세요"` |

### C. gluegen — @component 코드젠 제거

| 파일 | 변경 |
|------|------|
| `internal/gluegen/gluegen.go` | `collectComponents`, `collectComponentsForDomain` 함수 삭제. `transformSource`에서 component 치환 로직 삭제. components 파라미터 제거 |
| `internal/gluegen/server.go` | `generateServerStruct`에서 components 파라미터·Handler struct component 필드 생성 로직 삭제 |
| `internal/gluegen/domain.go` | `transformServiceFilesWithDomains`에서 components 관련 로직 삭제. Handler struct component 필드 생성 삭제 |

### D. manual-for-ai.md 문서 정리

| 위치 | 변경 |
|------|------|
| SSaC 문법 `@component` 태그 | 삭제 |
| 10 Sequence Types 표 `call` 행 | `@component or @func` → `@func` |
| `@component notification` 예시 | 삭제 |
| model/*.go Rules | `@component` 참조 설명 삭제. `// @dto` 설명만 유지 |
| SSOT Connection Map | `@component` 연결선 제거 |

### E. README.md

| 위치 | 변경 |
|------|------|
| Cross-Validation 목록 | `@component` 관련 항목 없음 (변경 불필요) |

## 변경 파일 목록

| 파일 | 변경 |
|------|------|
| `specs/dummy-lesson/service/enrollment/enroll_course.go` | notification call 시퀀스 삭제 |
| `specs/dummy-lesson/model/notification.go` | **삭제** |
| `internal/crosscheck/crosscheck.go` | @component ERROR 검증 추가 |
| `internal/gluegen/gluegen.go` | component 관련 함수·파라미터 삭제 |
| `internal/gluegen/server.go` | components 파라미터·로직 삭제 |
| `internal/gluegen/domain.go` | components 관련 로직 삭제 |
| `artifacts/manual-for-ai.md` | @component 문서 전부 삭제 |

## 의존성

- ssac 수정지시서014 (ssac 측 @component 파서 제거) — 병렬 진행 가능. fullend가 crosscheck ERROR로 먼저 차단

## 검증 방법

```bash
# 1. fullend 빌드
go build ./cmd/fullend/

# 2. validate — @component 없는 스펙 통과 확인
fullend validate specs/dummy-lesson

# 3. gen + build + hurl 통과
fullend gen specs/dummy-lesson /tmp/dummy-lesson-out
cd /tmp/dummy-lesson-out/backend && go mod tidy && go build ./...

# 4. go test
go test ./internal/...
```

