# Phase 019: pagination 응답 end-to-end 연결

## 배경

OpenAPI에 `x-pagination`이 정의된 List 엔드포인트(예: `ListGigs`)가 pagination 메타를 반환하지 않는다. smoke test에서 `$.total` assertion 실패.

## 설계 결정

### 제네릭 래퍼 패턴

엔티티별 DTO 대신 제네릭 래퍼 2종으로 모든 pagination을 처리한다.

| x-pagination style | 제네릭 타입 | 용도 |
|---|---|---|
| `offset` | `Page[T]` | 총 개수 기반 페이지네이션 |
| `cursor` | `Cursor[T]` | 무한 스크롤, 대용량 |
| 없음 | `[]T` | 페이지네이션 불필요 |

### 책임 경계

| 역할 | 담당 |
|---|---|
| `Page[T]`, `Cursor[T]` 타입 정의 | **fullend** `pkg/pagination/` |
| `@get Page[Gig]` 파싱 + `x-pagination` 교차 검증 | **SSaC** (수정지시서 014) |
| `@response gigPage` 간단쓰기 파싱 | **SSaC** (수정지시서 014) |
| handler 코드 생성 | **SSaC** generator |
| model interface + impl + struct 생성 | **fullend** gluegen |
| DTO(Page/Cursor) 자동 import 배선 | **fullend** gluegen |

### SSaC 스펙 문법

```go
// 페이지네이션 목록 — 간단쓰기
// @get Page[Gig] gigPage = Gig.List({Query: query})
// @response gigPage

// 단건 — 풀어쓰기
// @get Gig gig = Gig.FindByID({ID: id})
// @response {
//   gig: gig
// }
```

### 생성 코드 기대값

model interface:
```go
List(opts QueryOpts) (*pagination.Page[Gig], error)
```

handler:
```go
gigPage, err := model.List(opts)
c.JSON(200, gigPage)
```

응답:
```json
{
  "items": [...],
  "total": 42
}
```

## 변경 작업

### 1. fullend: `pkg/pagination/` 생성

```go
// pkg/pagination/page.go
type Page[T any] struct {
    Items []T   `json:"items"`
    Total int64 `json:"total"`
}

// pkg/pagination/cursor.go
type Cursor[T any] struct {
    Items      []T    `json:"items"`
    NextCursor string `json:"next_cursor"`
    HasNext    bool   `json:"has_next"`
}
```

### 2. SSaC: 수정지시서 014 ✅ 완료

- `parseInputs()` 콜론 없는 입력 → ERROR
- `Page[T]`, `Cursor[T]` 제네릭 타입 파싱 (`Result.Wrapper` 필드)
- `@response 변수명` 간단쓰기 지원 (`response_direct` 템플릿)
- `x-pagination` ↔ `Page[T]`/`Cursor[T]` 교차 검증
  - `offset` + `Page[T]` 아님 → ERROR
  - `cursor` + `Cursor[T]` 아님 → ERROR
  - 없음 + `Page[T]`/`Cursor[T]` 사용 → ERROR ([]T 사용 의무)

### 3. fullend: model_impl `Page[T]` 반환 지원

`internal/gluegen/model_impl.go` 수정:
- List 메서드가 `x-pagination: offset`이면 `*pagination.Page[T]` 반환
- COUNT 쿼리 + SELECT LIMIT/OFFSET → `Page[T]{Items: rows, Total: count}` 구성
- `x-pagination: cursor`면 `*pagination.Cursor[T]` 반환

### 4. fullend: model interface 생성 수정

`internal/gluegen/` 에서 model interface 생성 시:
- `x-pagination: offset` → `List(opts QueryOpts) (*pagination.Page[Gig], error)`
- `x-pagination: cursor` → `List(opts QueryOpts) (*pagination.Cursor[Gig], error)`
- 없음 → `List() ([]Gig, error)`

### 5. dummy-gigbridge SSaC 스펙 업데이트

```go
// specs/dummy-gigbridge/service/gig/list_gigs.go
// @get Page[Gig] gigPage = Gig.List({Query: query})
// @response gigPage
func ListGigs() {}
```

### 6. hurl-gen pagination assertion

`x-pagination` 있으면:
- `$.total exists` assertion 생성
- `$.items isCollection` assertion 생성

## 의존성

- **SSaC 수정지시서 014** — Page[T] 파싱 + @response 간단쓰기 (선행 필수)
- fullend model_impl — QueryOpts 기반 dynamic SQL (기존 구현 완료)
- fullend `pkg/pagination/` — 타입 정의 (이 Phase에서 생성)

## 검증 방법

```bash
fullend gen specs/dummy-gigbridge artifacts/dummy-gigbridge
```

1. `pkg/pagination/` 에 `Page[T]`, `Cursor[T]` 타입 존재
2. `models_gen.go`에서 `List(opts QueryOpts) (*pagination.Page[Gig], error)` 시그니처
3. `list_gigs.go` 핸들러에서 `c.JSON(200, gigPage)` 확인
4. `go build ./...` 통과
5. `DISABLE_AUTHZ=1 DISABLE_STATE_CHECK=1` 서버 시작 → `hurl --test smoke.hurl` 전체 통과

## 상태: ✅ 완료
