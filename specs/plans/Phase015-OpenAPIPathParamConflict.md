# Phase 015: OpenAPI path param 이름 충돌 검증

## 배경

`/gigs/{ID}`와 `/gigs/{GigID}/proposals`가 공존하면 Gin 라우터가 런타임 panic을 발생시킨다.
같은 path segment 위치에서 param 이름이 다르면(`{ID}` vs `{GigID}`) 라우터 트리 충돌이 발생하기 때문이다.

fullend validate 단계에서 이를 감지하면 `go build` → 런타임 panic까지 가지 않고 SSOT 수정 단계에서 해결 가능.

## 목표

OpenAPI 검증에 path param 이름 충돌 검사 추가. 같은 path segment 위치에 서로 다른 param 이름이 있으면 ERROR.

## 검증 규칙

1. 모든 OpenAPI path를 segment 단위로 분리
2. 각 segment 위치별로 path param(`{...}`)을 수집
3. 같은 위치에 2개 이상의 서로 다른 param 이름이 있으면 ERROR

예시:
```
/gigs/{ID}              → segment[1] = {ID}
/gigs/{ID}/publish      → segment[1] = {ID}
/gigs/{GigID}/proposals → segment[1] = {GigID}  ← 충돌!
```

에러 메시지:
```
[ERROR] OpenAPI path param 충돌: segment[1]에 {ID}와 {GigID}가 혼재 — 이름을 통일하세요
```

## 변경 파일

### `internal/orchestrator/validate.go` — OpenAPI 검증 단계에 추가

```go
func checkPathParamConflicts(doc *openapi3.T) []string {
    // segmentParams: map[int]map[string][]string  — position → paramName → []paths
    // 같은 position에 2개 이상의 paramName이 있으면 ERROR
}
```

### 테스트

- `TestCheckPathParamConflicts_Conflict` — `{ID}` vs `{GigID}` 감지
- `TestCheckPathParamConflicts_NoConflict` — 모두 `{ID}` 사용 시 통과

## 의존성

없음. OpenAPI 파싱 결과만 사용.

## 검증 방법

```bash
go test ./internal/orchestrator/ -run TestCheckPathParam -v
```

충돌 있는 OpenAPI로 `fullend validate` → ERROR 출력 확인.

## 상태: ✅ 완료
