✅ 완료

# Phase 009: pkg/middleware 삭제 — 생성 미들웨어로 단일화

## 목표

`pkg/middleware/bearerauth.go`를 삭제하고, 인증이 필요한 프로젝트는 반드시 claims config를 통해 gluegen이 생성하는 미들웨어만 사용하도록 단일화한다.

## 배경

Phase 006에서 claims config 기반 미들웨어 생성을 도입했으나, claims 없는 프로젝트의 fallback으로 `pkg/middleware`를 남겨뒀다. 문제점:

1. **fallback이 위험** — claims 설정 누락 시 조용히 `pkg/middleware`로 빠지고, `*middleware.CurrentUser` vs `*model.CurrentUser` 타입 불일치 panic 발생
2. **claims 없이 인증이 필요한 경우는 없음** — `fullend.yaml`에 claims가 없으면 인증 자체가 없는 프로젝트이므로 BearerAuth 미들웨어 불필요
3. **코드 혼란** — 같은 이름(`BearerAuth`), 같은 구조의 함수가 두 패키지에 존재

## 변경 사항

### 1. `pkg/middleware/bearerauth.go` — 삭제

파일 삭제. `CurrentUser` struct도 함께 제거.

### 2. `internal/gluegen/domain.go` — fallback 분기 제거

```go
// 변경 전 (320-324행)
if len(claims) > 0 {
    imports = append(imports, fmt.Sprintf("\"%s/internal/middleware\"", modulePath))
} else {
    imports = append(imports, "\"github.com/geul-org/fullend/pkg/middleware\"")
}

// 변경 후
imports = append(imports, fmt.Sprintf("\"%s/internal/middleware\"", modulePath))
```

`claims` 파라미터는 `hasBearer` 조건 안에 있으므로, bearer 인증이 있으면 항상 생성 미들웨어를 사용.

### 3. `internal/gluegen/gluegen.go` — claims 없이 bearer 있는 경우 에러

`Generate()`에서 OpenAPI에 bearerAuth 스킴이 있는데 claims config가 없으면 검증 에러를 반환:

```go
if hasBearerScheme(doc) && len(claims) == 0 {
    return fmt.Errorf("OpenAPI has bearerAuth security but fullend.yaml has no claims config")
}
```

빌드 시점 에러로 잡아서 런타임 panic을 원천 방지.

### 4. `artifacts/manual-for-ai.md` — 문서 업데이트

`pkg/middleware.BearerAuth` fallback 설명 제거. claims config 필수 명시.

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `pkg/middleware/bearerauth.go` | 삭제 |
| `internal/gluegen/domain.go` | 수정 — fallback import 분기 제거 |
| `internal/gluegen/gluegen.go` | 수정 — bearer + no claims 검증 에러 추가 |
| `artifacts/manual-for-ai.md` | 수정 — fallback 설명 제거 |

## 의존성

- Phase 006 완료 (✅) — 생성 미들웨어 기반

## 검증 방법

```bash
go build ./cmd/fullend/
# claims 있는 프로젝트 — 정상 생성
./fullend gen specs/dummy-lesson artifacts/dummy-lesson
cd artifacts/dummy-lesson/backend && go build ./...

# claims 없이 bearerAuth 사용 시 — 에러 확인
# (fullend.yaml에서 claims 제거 후 gen → 에러 메시지 확인)
```
