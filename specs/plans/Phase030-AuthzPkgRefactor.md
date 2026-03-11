# ✅ 완료 Phase 030: pkg/authz 리팩터 — 환경변수 Rego 로딩 + data.owners DB 조회 + authzgen 코드젠 제거

## 배경

BUG-1 (CRITICAL): authz-gen이 OPA data.owners를 DB에서 로딩하지 않아, 소유권 기반 정책이 항상 실패.

근본 원인: `authzgen.go`가 `pkg/authz`를 무시하고 Go 코드를 문자열로 새로 생성하면서, input 구조 불일치 + data.owners 로딩 누락 발생.

## 현재 문제점

### 1. authzgen.go가 pkg/authz를 복제하면서 분기

| 항목 | pkg/authz | authzgen.go 생성 코드 | Rego 기대 |
|---|---|---|---|
| user 키 | `input.user.id` | `input.claims.user_id` | `input.claims.user_id` |
| role | 없음 | `input.claims.role` | `input.claims.role` |
| resource_id | `input.resource_id` | `input.resource_owner_id` | `input.resource_id` |

### 2. data.owners 미로딩

Check()가 OPA에 input만 전달. `data.owners.gig[resource_id]` 참조 시 항상 undefined → deny.

### 3. Role 미전달

SSaC `@auth` 코드젠이 `Role` 필드를 생성하지 않음 → Rego `input.claims.role` 항상 빈값.

## 설계 결정

- **pkg/authz가 유일한 authz 구현** — authzgen의 Go 코드 생성 제거
- **`OPA_POLICY_PATH` 환경변수**로 .rego 파일 경로 주입 (embed 제거)
- **`@ownership` 기반 DB 쿼리**를 pkg/authz.Init()에서 등록, Check()에서 실행
- artifacts에서 `fullend/pkg/authz` 직접 import (다른 pkg/와 동일 패턴)

## 환경변수

| 환경변수 | 용도 | 기본값 | 예시 |
|---|---|---|---|
| `OPA_POLICY_PATH` | Rego 정책 파일 경로 | (필수) | `/etc/app/authz.rego` |
| `DISABLE_AUTHZ` | 인가 우회 (기존 유지) | 없음 | `1` |

## 변경 내용

### 1. `pkg/authz/authz.go` — 리팩터

**Init 시그니처 변경:**
```go
func Init(db *sql.DB, ownerships []OwnershipMapping) error
```
- `OPA_POLICY_PATH` 환경변수에서 rego 파일 경로 읽기
- `os.ReadFile`로 rego 로딩 (embed 제거)
- `ownerships` 저장 (Check에서 DB 쿼리용)

**CheckRequest 보강:**
```go
type CheckRequest struct {
    Action     string
    Resource   string
    UserID     int64
    Role       string   // 추가
    ResourceID int64
}
```

**input 구조 Rego 기준 통일:**
```go
opaInput := map[string]interface{}{
    "claims":      map[string]interface{}{"user_id": req.UserID, "role": req.Role},
    "action":      req.Action,
    "resource":    req.Resource,
    "resource_id": req.ResourceID,
}
```

**data.owners 로딩:**
```go
// Check() 내부, OPA 평가 전
owners := map[string]interface{}{}
for _, om := range globalOwnerships {
    if om.Resource == req.Resource {
        // SELECT om.Column FROM om.Table WHERE id = req.ResourceID
        var ownerID int64
        row := globalDB.QueryRow(
            fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", om.Column, om.Table),
            req.ResourceID,
        )
        if err := row.Scan(&ownerID); err == nil {
            owners[om.Resource] = map[string]interface{}{
                fmt.Sprint(req.ResourceID): ownerID,
            }
        }
    }
}

// OPA 평가 시 data로 전달
rego.EvalInput(opaInput),
rego.EvalData(map[string]interface{}{"owners": owners}),
```

**OwnershipMapping 타입 (pkg/authz에 추가):**
```go
type OwnershipMapping struct {
    Resource string // "gig", "proposal"
    Table    string // "gigs", "proposals"
    Column   string // "client_id", "freelancer_id"
}
```

### 2. `pkg/authz/authz.rego` — 삭제

embed용 더미 rego 파일 제거. 런타임에 `OPA_POLICY_PATH`에서 로딩.

### 3. `internal/gluegen/authzgen.go` — 코드젠 축소

**변경 전:** .rego 복사 + Go 코드 전체 생성
**변경 후:** .rego 파일 복사만

- `generateDefaultAuthzSource()` 함수 삭제
- `GenerateAuthzPackage()`에서 Go 코드 생성 분기 제거
- authzPackage 유무와 무관하게 .rego 복사만 수행

### 4. `internal/gluegen/` — backend 코드젠에서 import 경로 변경

artifacts의 service 코드가 `internal/authz` 대신 `fullend/pkg/authz` import하도록:
- main.go의 `authz.Init(db)` → `authz.Init(db, ownerships)`
- service 코드의 import: `github.com/gigbridge/api/internal/authz` → `github.com/geul-org/fullend/pkg/authz`

### 5. SSaC 수정지시서 — Role 전달

SSaC `@auth` 코드젠이 `Role: currentUser.Role`을 CheckRequest에 추가해야 함.
→ `~/.clari/repos/ssac/files/수정지시서v2/` 에 수정지시서 작성.

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `pkg/authz/authz.go` | Init 시그니처, input 구조, data.owners 로딩, Role 추가 |
| `pkg/authz/authz.rego` | 삭제 |
| `internal/gluegen/authzgen.go` | Go 코드 생성 제거, .rego 복사만 |
| `internal/gluegen/backend_gen.go` (추정) | import 경로 변경, Init 호출 변경 |

## 의존성

- SSaC `@auth` 코드젠에서 Role 전달 (수정지시서 필요)
- `internal/policy.OwnershipMapping` → `pkg/authz.OwnershipMapping` 변환 필요 (gluegen에서 main.go 생성 시)

## 검증 방법

1. `go test ./pkg/authz/...` 통과
2. `go build ./...` 통과
3. `fullend gen specs/gigbridge artifacts/gigbridge` → 생성된 코드에 `internal/authz/authz.go` 없음, `fullend/pkg/authz` import 확인
4. gigbridge `go build` 통과
5. `DISABLE_AUTHZ=0 OPA_POLICY_PATH=./authz.rego` 로 서버 기동 → 소유권 기반 인가 동작 확인
