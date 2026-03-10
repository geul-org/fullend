✅ 완료

# Phase 008: CanTransition 반환 타입 bool → error 통일

## 목표

`stategen.go`가 생성하는 `CanTransition` 함수의 반환 타입을 `bool` → `error`로 변경한다. SSaC 수정지시서 004 완료에 맞춰 fullend 측을 통일한다.

## 배경

- SSaC `go_templates.go`가 `err := ...state.CanTransition(...); err != nil` 패턴으로 변경 완료 (수정지시서 004)
- fullend `stategen.go`는 아직 `bool` 반환 → SSaC 생성 코드와 불일치
- `error` 반환 시 전이 실패 사유("cannot transition from draft via archive")를 API 응답에 전달 가능

## 변경 사항

### 1. `internal/gluegen/stategen.go` — CanTransition 반환 변경

**string 기반 상태 (기본)**:
```go
// 변경 전
func CanTransition(input Input, event string) bool {
    status, _ := input.Status.(string)
    _, ok := transitions[transitionKey{from: status, event: event}]
    return ok
}

// 변경 후
func CanTransition(input Input, event string) error {
    status, _ := input.Status.(string)
    _, ok := transitions[transitionKey{from: status, event: event}]
    if !ok {
        return fmt.Errorf("cannot transition from %q via %q", status, event)
    }
    return nil
}
```

**bool 기반 상태** (`generateBoolCanTransition`):
```go
// 변경 전
func CanTransition(input Input, event string) bool {
    current := resolveState(input.Status)
    _, ok := transitions[transitionKey{from: current, event: event}]
    return ok
}

// 변경 후
func CanTransition(input Input, event string) error {
    current := resolveState(input.Status)
    _, ok := transitions[transitionKey{from: current, event: event}]
    if !ok {
        return fmt.Errorf("cannot transition from %q via %q", current, event)
    }
    return nil
}
```

생성 코드에 `"fmt"` import 추가.

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `internal/gluegen/stategen.go` | 수정 — CanTransition 반환 `bool` → `error`, `fmt` import 추가 |

## 의존성

- SSaC 수정지시서 004 완료 (✅)

## 검증 방법

```bash
go build ./cmd/fullend/
./fullend gen specs/dummy-lesson artifacts/dummy-lesson
# artifacts/dummy-lesson/backend/internal/states/coursestate/state.go 확인:
#   func CanTransition(input Input, event string) error
cd artifacts/dummy-lesson/backend && go build ./...
```
