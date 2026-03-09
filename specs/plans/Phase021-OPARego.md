# Phase 021: OPA Rego 인가 정책 SSOT

## 목표

`specs/<project>/policy/*.rego`에 인가 정책을 선언한다. fullend가 교차 검증하고, OPA Go SDK 기반 Authorizer 구현체를 자동 생성하여 기존 `authorize` 시퀀스의 스텁을 실제 정책 엔진으로 대체한다.

```
현재:
  SSaC authorize → authz.Check(user, action, resource, id) → 항상 true 스텁
  인가 정책 = 암묵적 (코드에 흩어져 있거나 없음)

목표:
  policy/*.rego → OPA 기반 Authorizer 구현체 자동 생성
  fullend validate가 Policy ↔ SSaC/OpenAPI 교차 검증
  Authorizer.Check가 실제 Rego 정책을 평가
```

---

## Rego 정책 형식 (제약된 패턴)

표준 Rego의 전체 기능 대신, fullend가 파싱·검증·코드젠할 수 있는 고정 패턴만 사용한다.

### input 스키마 (고정)

```json
{
  "user": {"id": 123, "role": "instructor"},
  "action": "update",
  "resource": "course",
  "resource_id": 456,
  "resource_owner": 789
}
```

| 필드 | 출처 | 설명 |
|---|---|---|
| `input.user.id` | `CurrentUser.UserID` | 인증된 사용자 ID |
| `input.user.role` | `CurrentUser.Role` | 사용자 역할 |
| `input.action` | SSaC `@action` | 수행할 동작 |
| `input.resource` | SSaC `@resource` | 대상 리소스 |
| `input.resource_id` | SSaC `@id` | 리소스 식별자 |
| `input.resource_owner` | ownership 매핑 → DB 조회 | 리소스 소유자 ID |

### 허용 패턴

```rego
package authz

import rego.v1

default allow := false

# 1. 무조건 허용 (인증만 되면)
allow if {
    input.action == "create"
    input.resource == "course"
}

# 2. 역할 기반
allow if {
    input.action == "create"
    input.resource == "course"
    input.user.role == "instructor"
}

# 3. 소유자 기반
allow if {
    input.action == "update"
    input.resource == "course"
    input.user.id == input.resource_owner
}

# 4. 역할 + 소유자
allow if {
    input.action == "delete"
    input.resource == "course"
    input.user.role == "instructor"
    input.user.id == input.resource_owner
}

# 5. 복수 액션 (set)
allow if {
    input.action in {"update", "delete", "publish"}
    input.resource == "course"
    input.user.id == input.resource_owner
}
```

### ownership 매핑 (구조화 주석)

Rego 파일 상단에 fullend 전용 주석으로 소유자 조회 방법을 선언한다:

```rego
# @ownership course: courses.instructor_id
# @ownership lesson: courses.instructor_id via lessons.course_id
# @ownership review: reviews.user_id
```

| 형식 | 의미 |
|---|---|
| `resource: table.column` | 직접 조회. `SELECT column FROM table WHERE id = ?` |
| `resource: table.column via join_table.fk` | JOIN 조회. 중간 테이블을 거쳐 소유자 조회 |

`input.resource_owner`가 정책에서 참조되지 않는 리소스는 ownership 선언 불필요.

---

## 예시: dummy-lesson

### `specs/dummy-lesson/policy/authz.rego`

```rego
package authz

import rego.v1

# @ownership course: courses.instructor_id
# @ownership lesson: courses.instructor_id via lessons.course_id
# @ownership review: reviews.user_id

default allow := false

# 강의 생성: 인증된 사용자 누구나
allow if {
    input.action == "create"
    input.resource == "course"
}

# 강의 수정/삭제/공개: 강의 소유자만
allow if {
    input.action in {"update", "delete", "publish"}
    input.resource == "course"
    input.user.id == input.resource_owner
}

# 레슨 생성/수정/삭제: 강의 소유자만
allow if {
    input.action in {"create", "update", "delete"}
    input.resource == "lesson"
    input.user.id == input.resource_owner
}

# 수강 등록: 인증된 사용자 누구나
allow if {
    input.action == "enroll"
    input.resource == "course"
}

# 리뷰 삭제: 리뷰 작성자만
allow if {
    input.action == "delete"
    input.resource == "review"
    input.user.id == input.resource_owner
}
```

---

## 교차 검증 규칙

### Policy ↔ SSaC

| 규칙 | 수준 |
|---|---|
| SSaC `authorize`의 (action, resource) 쌍 → Rego에 매칭 `allow` 규칙 존재 | WARNING |
| Rego `allow` 규칙의 (action, resource) 쌍 → SSaC에 매칭 `authorize` 시퀀스 존재 | WARNING |
| Rego에서 `input.resource_owner` 참조 → 해당 resource의 `@ownership` 주석 존재 | ERROR |

### Policy ↔ OpenAPI

| 규칙 | 수준 |
|---|---|
| SSaC `authorize` 있는 operationId → OpenAPI에 `security` 선언 존재 | WARNING |

### Policy ↔ DDL

| 규칙 | 수준 |
|---|---|
| `@ownership` 주석의 table.column → DDL에 해당 테이블·컬럼 존재 | ERROR |
| `@ownership` via join의 join_table.fk → DDL에 해당 테이블·컬럼 존재 | ERROR |

### Policy ↔ States

stateDiagram과 Rego는 독립적 관심사(상태 vs 권한)이므로 런타임 데이터를 주고받지 않는다. 교차 검증만 수행한다.

| 규칙 | 수준 |
|---|---|
| stateDiagram 전이 이벤트에 대응하는 SSaC `authorize`가 있으면 → Rego에 매칭 `allow` 규칙 존재 | WARNING |

---

## 코드젠 출력

### 디렉토리 구조

```
<artifacts-dir>/
  backend/
    internal/
      authz/
        authz.go          # OPA Authorizer 구현체
        policy.rego        # specs에서 복사 (embed용)
```

### 생성 코드: `authz/authz.go`

```go
package authz

import (
    "context"
    "database/sql"
    _ "embed"
    "fmt"

    "github.com/open-policy-agent/opa/rego"

    "<module>/internal/model"
)

//go:embed policy.rego
var policyRego string

type OPAAuthorizer struct {
    query rego.PreparedEvalQuery
    db    *sql.DB
}

func New(db *sql.DB) (*OPAAuthorizer, error) {
    query, err := rego.New(
        rego.Query("data.authz.allow"),
        rego.Module("policy.rego", policyRego),
    ).PrepareForEval(context.Background())
    if err != nil {
        return nil, fmt.Errorf("OPA 초기화 실패: %w", err)
    }
    return &OPAAuthorizer{query: query, db: db}, nil
}

func (a *OPAAuthorizer) Check(user *model.CurrentUser, action, resource string, id interface{}) (bool, error) {
    input := map[string]interface{}{
        "user":        map[string]interface{}{"id": user.UserID, "role": user.Role},
        "action":      action,
        "resource":    resource,
        "resource_id": id,
    }

    // ownership 조회 (resource별 생성된 코드)
    if ownerID, err := a.lookupOwner(resource, id); err == nil {
        input["resource_owner"] = ownerID
    }

    results, err := a.query.Eval(context.Background(), rego.EvalInput(input))
    if err != nil {
        return false, fmt.Errorf("OPA 평가 실패: %w", err)
    }
    if len(results) == 0 {
        return false, nil
    }
    allowed, ok := results[0].Expressions[0].Value.(bool)
    return ok && allowed, nil
}

// lookupOwner — @ownership 주석에서 파생된 DB 조회
func (a *OPAAuthorizer) lookupOwner(resource string, id interface{}) (int64, error) {
    switch resource {
    case "course":
        var ownerID int64
        err := a.db.QueryRow("SELECT instructor_id FROM courses WHERE id = $1", id).Scan(&ownerID)
        return ownerID, err
    case "lesson":
        var ownerID int64
        err := a.db.QueryRow(
            "SELECT c.instructor_id FROM courses c JOIN lessons l ON c.id = l.course_id WHERE l.id = $1", id,
        ).Scan(&ownerID)
        return ownerID, err
    case "review":
        var ownerID int64
        err := a.db.QueryRow("SELECT user_id FROM reviews WHERE id = $1", id).Scan(&ownerID)
        return ownerID, err
    default:
        return 0, fmt.Errorf("unknown resource: %s", resource)
    }
}
```

### main.go 연동

기존 스텁 Authorizer 대신 OPA Authorizer를 주입:

```go
// 기존 (스텁)
server.Authz = &StubAuthorizer{}

// 변경 (OPA)
authorizer, err := authz.New(db)
if err != nil { log.Fatal(err) }
server.Authz = authorizer
```

---

## 구현

### 새 파일

| 파일 | 역할 |
|---|---|
| `artifacts/internal/policy/parser.go` | .rego 파서 (ownership 주석 + action/resource 쌍 추출) |
| `artifacts/internal/policy/types.go` | Policy, OwnershipMapping, AllowRule 구조체 |
| `artifacts/internal/policy/parser_test.go` | 파서 테스트 |
| `artifacts/internal/crosscheck/policy.go` | Policy ↔ SSaC/OpenAPI/DDL 교차 검증 |
| `artifacts/internal/gluegen/authzgen.go` | .rego → authz/ 패키지 생성 |

### 수정 파일

| 파일 | 변경 |
|---|---|
| `artifacts/internal/orchestrator/detect.go` | `KindPolicy` + `policy/*.rego` 감지 |
| `artifacts/internal/orchestrator/validate.go` | policy 검증 단계 추가 |
| `artifacts/internal/orchestrator/gen.go` | authz-gen 코드젠 단계 추가 |
| `artifacts/internal/crosscheck/crosscheck.go` | `Policies` 필드 + `CheckPolicy` 호출 |
| `artifacts/internal/gluegen/gluegen.go` | main.go에서 OPA Authorizer 주입 코드 생성 |

### 더미 데이터

| 파일 | 역할 |
|---|---|
| `specs/dummy-lesson/policy/authz.rego` | 강의 플랫폼 인가 정책 |

---

## OPA Go SDK 의존성

```bash
go get github.com/open-policy-agent/opa
```

OPA Go SDK는 Rego 파일을 Go 프로세스 내에서 평가한다. 별도 OPA 서버 불필요.

---

## Rego 파서 설계

fullend는 Rego를 실행하지 않고, 정적 분석만 수행한다:

### 1. ownership 주석 파싱

```go
var reOwnership = regexp.MustCompile(
    `^#\s*@ownership\s+(\w+):\s+(\w+)\.(\w+)(?:\s+via\s+(\w+)\.(\w+))?$`,
)
```

### 2. allow 규칙에서 (action, resource) 쌍 추출

```go
var reAction = regexp.MustCompile(`input\.action\s*(?:==\s*"(\w+)"|in\s*\{([^}]+)\})`)
var reResource = regexp.MustCompile(`input\.resource\s*==\s*"(\w+)"`)
var reOwnerRef = regexp.MustCompile(`input\.resource_owner`)
```

### 3. Rego 구문 검증

OPA Go SDK의 `ast.CompileModules()`로 Rego 구문 유효성 검증. fullend가 자체 Rego 파서를 만들지 않는다.

---

## 문서 업데이트

### CLAUDE.md

SSOT 테이블에 추가:

```
| 인가 정책 | `<root>/policy/*.rego` | OPA Rego |
```

교차 검증 규칙에 `Policy ↔ SSaC`, `Policy ↔ DDL` 섹션 추가.

gen 단계에 `authz-gen` 추가.

### manual-for-ai.md

디렉토리 구조에 `policy/*.rego` 추가.

OPA Rego 정책 섹션 신설:
- 고정 input 스키마
- 허용 패턴 5가지
- @ownership 주석 문법

SSOT 연결 맵에 Policy 추가.

### README.md

SSOT 목록에 `policy/*.rego` 추가. 6→7개 SSOT.

---

## 의존성

- **Phase020 완료** ✅ (stateDiagram, guard state)
- **수정지시서009 완료 필요** — SSaC authorize의 `@message` 지원 (Phase021 동작에 직접 영향 없으나, 완성도를 위해)
- **OPA Go SDK**: `github.com/open-policy-agent/opa` (Apache 2.0)
- **kin-openapi**: 이미 사용 중

## 검증

```bash
# 1. policy 파일 작성
cat specs/dummy-lesson/policy/authz.rego

# 2. fullend validate
fullend validate specs/dummy-lesson
# ✓ Policy       1 files, 6 rules, 3 ownership mappings

# 3. fullend gen
fullend gen specs/dummy-lesson artifacts/dummy-lesson

# 4. 생성된 authz 패키지 확인
cat artifacts/dummy-lesson/backend/internal/authz/authz.go
cat artifacts/dummy-lesson/backend/internal/authz/policy.rego

# 5. (서버 실행 후) 인가 테스트
# - 강의 소유자가 수정 → 200
# - 비소유자가 수정 → 403
# - 인증 없이 삭제 → 401
```
