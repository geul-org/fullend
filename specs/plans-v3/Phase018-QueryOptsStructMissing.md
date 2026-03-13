# Phase018: QueryOpts struct 누락 — codegen 빌드 실패 수정 ✅ 완료

## 목표

`fullend gen`으로 생성된 backend가 `go build` 시 `undefined: QueryOpts` 에러로 실패하는 버그를 수정한다.

## 버그 요약

| 항목 | 내용 |
|------|------|
| 증상 | `artifacts/*/backend/internal/model/queryopts.go`에서 `undefined: QueryOpts` (4건) |
| 원인 | `generateQueryOpts()` 템플릿이 `QueryOpts` 타입을 사용하지만 정의하지 않음 |
| 발생 시점 | **처음부터** — `type QueryOpts struct` 없음 |
| 발견 경로 | dummy-zenflow `fullend gen` → `go build` 실패 |

## 근본 원인 분석

`QueryOpts` struct의 책임 소재가 **SSaC generator와 fullend gluegen 사이에서 분리**되어 있었다.

| 역할 | 담당 | 파일 |
|------|------|------|
| `type QueryOpts struct` **정의** | SSaC generator | `internal/ssac/generator/go_interface.go:345-354` |
| `QueryOpts` **사용** (ParseQueryOpts, BuildSelectQuery, BuildCountQuery) | fullend gluegen | `internal/gluegen/queryopts.go` 템플릿 |

SSaC의 `go_interface.go:313` `renderInterfaces()`는 `needQueryOpts == true`일 때(pagination을 쓰는 모델이 있을 때)만 `type QueryOpts struct`를 인터페이스 파일에 출력한다.

- **pagination을 쓰는 프로젝트** (gigbridge) → SSaC가 struct를 생성 → 빌드 성공
- **pagination을 안 쓰는 프로젝트** (zenflow) → SSaC가 struct를 생성하지 않음 → fullend의 `queryopts.go`가 무조건 생성됨 → `undefined: QueryOpts` 빌드 실패

## 해법 (Phase016 통합 이후 단순화)

Phase016에서 SSaC가 fullend에 통합되었으므로, SSaC 수정지시서 없이 **직접 양쪽을 수정**한다.

**원칙**: `QueryOpts` struct는 **fullend gluegen이 항상 생성**하고, SSaC generator의 조건부 생성은 **제거**한다.

## 변경 파일

| 파일 | 변경 |
|------|------|
| `internal/gluegen/queryopts.go` | 템플릿에 `type QueryOpts struct` 정의 추가 |
| `internal/ssac/generator/go_interface.go` | `needQueryOpts` 분기·파라미터·`hasQueryOpts()` 함수 제거 |
| `internal/ssac/generator/go_target.go` | `renderInterfaces()` 호출에서 `hasQueryOpts(st)` 인자 제거 |

## 수정 내용

### 1. fullend gluegen — struct 항상 생성

`internal/gluegen/queryopts.go`의 `generateQueryOpts()` 템플릿 문자열에서 `FilterConfig` struct 뒤, `ParseQueryOpts` 함수 앞에 추가:

```go
// QueryOpts holds parsed query parameters for pagination, sort, and filter.
type QueryOpts struct {
	Limit   int
	Offset  int
	Cursor  string
	SortCol string
	SortDir string
	Filters map[string]string
}
```

### 2. SSaC generator — 조건부 생성 제거 + dead code 정리

`internal/ssac/generator/go_interface.go:345`의 `needQueryOpts` 분기에서 `type QueryOpts struct` 출력을 제거한다. fullend가 항상 생성하므로 SSaC가 중복 생성하면 컴파일 에러가 발생한다.

추가로 dead code 정리:
- `renderInterfaces()` 시그니처에서 `needQueryOpts bool` 파라미터 제거
- `go_target.go:44`의 `hasQueryOpts(st)` 인자 제거
- `hasQueryOpts()` 함수 삭제 (더 이상 사용처 없음)

## 실행 순서

1. `internal/gluegen/queryopts.go` 템플릿에 `type QueryOpts struct` 추가
2. `internal/ssac/generator/go_interface.go`에서 `needQueryOpts` 분기의 QueryOpts struct 출력 제거
3. `go build ./cmd/fullend/` 빌드 확인
4. `go test ./...` 전체 통과 확인
5. gigbridge 검증: `fullend gen` → `go build` 성공 확인 (중복 정의 없음)
6. zenflow 검증: `fullend gen` → `go build` 성공 확인 (struct 존재)

## 검증

```bash
go build ./cmd/fullend/
go test ./...
fullend gen specs/gigbridge artifacts/gigbridge
cd artifacts/gigbridge/backend && go build -o server ./cmd/
fullend gen specs/zenflow artifacts/zenflow
cd artifacts/zenflow/backend && go build -o server ./cmd/
```

## 리스크

| 리스크 | 대응 |
|--------|------|
| SSaC 측 needQueryOpts 분기 제거 후 다른 코드가 의존 | needQueryOpts는 struct 출력에만 사용, 메서드 시그니처에는 영향 없음 |
| SSaC/STML repo 미러 동기화 | Phase016 결정에 따라 fullend에서 복사 내려주는 방식 — 미러 시 반영 필요 |
