✅ 완료

# Phase 013: glue-gen 버그 수정 + Model 구현체 생성

## 목표

Phase012 코드젠 결과 검토에서 발견된 fullend glue-gen 이슈 3건을 수정한다.

1. `main.go` — `Server`, `Handler`가 `package internal`에 있으나 import 없음
2. `main.tsx` — `@tanstack/react-query`가 deps에 있으나 `QueryClientProvider` 미설정
3. **Model 구현체 누락** — ssac이 선언한 Model interface를 sqlc 쿼리로 구현하는 코드가 없음

---

## 1. main.go — internal 패키지 import 추가

### 문제

생성된 `backend/cmd/main.go`:

```go
package main

import (
    "database/sql"
    "flag"
    "log"
    "net/http"
    _ "github.com/lib/pq"
)

func main() {
    // ...
    server := &Server{          // ← undefined: Server
    handler := Handler(server)  // ← undefined: Handler
}
```

`Server`와 `Handler`는 `backend/internal/` 패키지에 생성되지만, main.go에 import가 없다.
또한 `backend/go.mod`가 없어 모듈 경로를 알 수 없다.

### 수정

`generateMain()`에 **모듈 경로** 파라미터를 추가한다.

```go
// 변경 전
func generateMain(artifactsDir string, models []string) error

// 변경 후
func generateMain(artifactsDir string, models []string, modulePath string) error
```

**모듈 경로 결정 방식:**

1. `GlueInput`에 `ModulePath string` 필드 추가
2. orchestrator `genGlue()`에서 결정:
   - `backend/go.mod`가 이미 있으면 → module 줄 파싱
   - 없으면 → 프로젝트 디렉토리명에서 파생 (예: `dummy-lesson` → `dummy-lesson/backend`)
3. `generateMain()`이 go.mod도 함께 생성

**생성 결과:**

```go
// backend/go.mod
module dummy-lesson/backend

go 1.22

require (
    github.com/lib/pq v1.10.9
    github.com/oapi-codegen/runtime v1.1.1
)
```

```go
// backend/cmd/main.go
package main

import (
    "database/sql"
    "flag"
    "log"
    "net/http"

    _ "github.com/lib/pq"

    internal "dummy-lesson/backend/internal"
    "dummy-lesson/backend/internal/db"
    "dummy-lesson/backend/internal/model"
)

func main() {
    // ...
    db, err := sql.Open(*dbDriver, *dsn)
    // ...
    queries := db.New(db)

    server := &internal.Server{
        courseModel:     model.NewCourseModel(queries),
        enrollmentModel: model.NewEnrollmentModel(queries),
        // ...
    }
    handler := internal.Handler(server)
}
```

### 구현

| 파일 | 변경 |
|---|---|
| `gluegen/gluegen.go` | `GlueInput`에 `ModulePath string` 추가 |
| `gluegen/main_go.go` | `generateMain()`에 modulePath 파라미터 추가, import 생성, `internal.` 접두어, go.mod 생성, model 초기화 |
| `orchestrator/gen.go` | `genGlue()`에서 모듈 경로 결정 → `GlueInput.ModulePath` 전달 |

---

## 2. main.tsx — QueryClientProvider 래핑

### 문제

`package.json`에 `@tanstack/react-query: ^5`가 있고, stml이 생성한 페이지에서 `useQuery`를 사용한다.
그러나 `main.tsx`에 `QueryClientProvider`가 없어 런타임 에러 발생:

```
No QueryClient set, use QueryClientProvider to set one
```

### 수정

`writeMainTSX()`에 stml 의존성 맵을 전달하여, `@tanstack/react-query`가 있으면 `QueryClientProvider`를 래핑한다.

```go
// 변경 전
func writeMainTSX(srcDir string) error

// 변경 후
func writeMainTSX(srcDir string, stmlDeps map[string]string) error
```

**생성 결과 (react-query가 deps에 있을 때):**

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'

const queryClient = new QueryClient()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
)
```

### 구현

| 파일 | 변경 |
|---|---|
| `gluegen/frontend.go` `writeMainTSX()` | stmlDeps 파라미터 추가, react-query 감지 시 Provider 래핑 |
| `gluegen/frontend.go` `generateFrontendSetup()` | `writeMainTSX(srcDir, stmlDeps)` 호출로 변경 |

---

## 3. Model 구현체 생성 — sqlc 쿼리를 이용한 interface 구현

### 문제

현재 gen 파이프라인의 gap:

```
sqlc generate → backend/internal/db/     (Go struct + query 함수)
ssac generate → backend/internal/model/  (Go interface 선언)
                                          ↕ 구현체 없음
```

ssac은 **계약(interface)** 만 선언하고, sqlc는 **raw DB 함수**만 생성한다.
이 둘을 연결하는 **Model 구현체**가 없어 서비스 코드가 동작하지 않는다.

sqlc가 생성하는 것:
```go
// db/query.sql.go
type Course struct { ID int64; Title string; ... }

func (q *Queries) FindByID(ctx context.Context, id int64) (Course, error)
func (q *Queries) List(ctx context.Context) ([]Course, error)
func (q *Queries) Create(ctx context.Context, arg CreateCourseParams) (Course, error)
```

ssac이 선언하는 것:
```go
// model/models_gen.go
type CourseModel interface {
    FindByID(courseID int64) (*Course, error)
    List(opts QueryOpts) ([]Course, int, error)
    Create(userID int64, title string, ...) (*Course, error)
}
```

### 수정

**model 패키지 안에** interface 구현체를 직접 생성한다. 별도 패키지(adapter 등)를 만들지 않는다.

```
backend/internal/model/
├── models_gen.go     ← interface 선언 (ssac 생성, 건드리지 않음)
├── types.go          ← type Course = db.Course (glue-gen 생성)
├── course.go         ← CourseModel 구현체 (glue-gen 생성)
├── enrollment.go     ← EnrollmentModel 구현체
├── lesson.go         ← LessonModel 구현체
├── payment.go        ← PaymentModel 구현체
├── review.go         ← ReviewModel 구현체
└── user.go           ← UserModel 구현체
```

#### 3-1. 타입 alias (`model/types.go`)

model 패키지에서 sqlc 생성 타입을 재사용한다:

```go
// model/types.go (glue-gen 생성)
package model

import "dummy-lesson/backend/internal/db"

// sqlc 생성 struct를 model 타입으로 재사용
type Course = db.Course
type Lesson = db.Lesson
type Enrollment = db.Enrollment
type Payment = db.Payment
type Review = db.Review
type User = db.User
```

이렇게 하면 `model.CourseModel`의 `*Course`가 `*db.Course`와 동일 타입이 된다.

#### 3-2. Model 구현체 (`model/{model}.go`)

각 interface를 구현하는 struct:

```go
// model/course.go (glue-gen 생성)
package model

import (
    "context"
    "database/sql"

    "dummy-lesson/backend/internal/db"
)

// courseModelImpl implements CourseModel.
type courseModelImpl struct {
    q *db.Queries
}

// NewCourseModel creates a CourseModel backed by sqlc queries.
func NewCourseModel(q *db.Queries) CourseModel {
    return &courseModelImpl{q: q}
}

func (m *courseModelImpl) FindByID(courseID int64) (*Course, error) {
    c, err := m.q.FindByID(context.Background(), courseID)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, nil
        }
        return nil, err
    }
    return &c, nil
}

func (m *courseModelImpl) List(opts QueryOpts) ([]Course, int, error) {
    items, err := m.q.List(context.Background())
    if err != nil {
        return nil, 0, err
    }
    // TODO: apply opts (pagination, sort, filter)
    return items, len(items), nil
}

func (m *courseModelImpl) Create(userID int64, title, description, category, level string, price int64) (*Course, error) {
    c, err := m.q.Create(context.Background(), db.CreateCourseParams{
        InstructorID: userID,
        Title:        title,
        Description:  description,
        Category:     category,
        Level:        level,
        Price:        int32(price),
    })
    if err != nil {
        return nil, err
    }
    return &c, nil
}

func (m *courseModelImpl) Update(courseID int64, title, description, category, level string, price int64) error {
    return m.q.Update(context.Background(), db.UpdateCourseParams{
        ID:          courseID,
        Title:       title,
        Description: description,
        Category:    category,
        Level:       level,
        Price:       int32(price),
    })
}

func (m *courseModelImpl) Delete(courseID int64) error {
    return m.q.Delete(context.Background(), courseID)
}

func (m *courseModelImpl) Publish(courseID int64) error {
    return m.q.Publish(context.Background(), courseID)
}
```

#### 3-3. 차이점 흡수 규칙

| 차이 | 처리 방식 |
|---|---|
| `context.Context` | 구현체에서 `context.Background()` 주입 |
| 값 → 포인터 | `c, err := q.FindByID(...)` → `return &c, nil` |
| `sql.ErrNoRows` → nil | FindBy* 메서드에서 ErrNoRows 감지 → `return nil, nil` |
| sqlc `Params` struct | Create/Update에서 개별 파라미터 → `db.CreateXxxParams{}` 변환 |
| QueryOpts | List 메서드에서 opts 무시, TODO 주석 (Phase013 범위) |
| `:exec` 반환 | Delete/Update/Publish — sqlc `error` 반환을 그대로 전달 |

#### 3-4. 생성 전략

구현체 코드 생성에 필요한 입력:

| 입력 | 출처 | 용도 |
|---|---|---|
| 모델 인터페이스 메서드 | `models_gen.go` (ssac 생성) | 구현할 메서드 시그니처 |
| sqlc 쿼리 함수 | `db/queries/` SQL 파일 파싱 | 메서드명 매칭, Params struct 이름 |
| DDL 컬럼 정보 | ssac `SymbolTable` | Params struct 필드 매핑, 타입 변환 |
| 모듈 경로 | `GlueInput.ModulePath` | import 경로 |

**메서드 매칭:**

sqlc 쿼리의 `-- name:` 주석과 model interface 메서드명이 직접 매칭된다:

```sql
-- name: FindByID :one       → FindByID()
-- name: List :many          → List()
-- name: ListByCourse :many  → ListByCourse()
-- name: Create :one         → Create()
-- name: Update :exec        → Update()
-- name: Delete :exec        → Delete()
```

매칭되지 않는 메서드는 `panic("not implemented")` stub을 생성한다.

### 구현

| 파일 | 변경 |
|---|---|
| `gluegen/model_impl.go` | ★ NEW: Model 구현체 코드 생성 (`generateModelImpls()`) |
| `gluegen/gluegen.go` | 수정: Generate()에 구현체 생성 단계 추가 |
| `gluegen/main_go.go` | 수정: main.go에서 `model.NewXxxModel(queries)` 초기화 |

---

## 변경 파일 목록

| 파일 | 변경 유형 |
|---|---|
| `gluegen/gluegen.go` | 수정: GlueInput에 ModulePath 추가, 구현체 생성 단계 추가 |
| `gluegen/main_go.go` | 수정: import + go.mod + model 초기화 |
| `gluegen/frontend.go` | 수정: writeMainTSX QueryClientProvider |
| `gluegen/model_impl.go` | ★ NEW: Model 구현체 코드 생성 |
| `orchestrator/gen.go` | 수정: 모듈 경로 결정 |

## 생성 산출물 (dummy-lesson 기준)

| 생성 파일 | 내용 |
|---|---|
| `backend/go.mod` | 모듈 선언 + 의존성 |
| `backend/internal/model/types.go` | sqlc 타입 alias |
| `backend/internal/model/course.go` | CourseModel 구현체 |
| `backend/internal/model/enrollment.go` | EnrollmentModel 구현체 |
| `backend/internal/model/lesson.go` | LessonModel 구현체 |
| `backend/internal/model/payment.go` | PaymentModel 구현체 |
| `backend/internal/model/review.go` | ReviewModel 구현체 |
| `backend/internal/model/user.go` | UserModel 구현체 |

## 변경하지 않는 파일

| 파일 | 이유 |
|---|---|
| `model/models_gen.go` | ssac 생성물 — glue-gen이 건드리지 않음 |
| `orchestrator/sqlc_config.go` | sqlc 실행 자체는 이미 동작 |
| `gluegen/server.go` | Server struct는 이미 정상 |
| `crosscheck/*` | 교차 검증은 SSOT 레벨이므로 무관 |

## 의존성

- **ssac 수정지시서006** (FindByID opts 오적용, ListLessons opts 누락, 파라미터명 오류, total 미사용) — 병행 가능
- `sqlc` 설치 필수 (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)

## 검증

1. `go build ./artifacts/cmd/... ./artifacts/internal/...` 성공
2. `fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/` 재실행
3. `backend/cmd/main.go`에 internal + model import 존재
4. `backend/go.mod` 생성 확인
5. `backend/internal/model/` 에 모델별 구현체 파일 존재
6. `backend/internal/model/types.go`에 sqlc 타입 alias 존재
7. `model.NewCourseModel(queries)`로 초기화, `CourseModel` interface 만족
8. `frontend/src/main.tsx`에 `QueryClientProvider` 래핑 확인

## QueryOpts 동적 쿼리 (향후 Phase)

Phase013에서는 List 메서드의 opts를 무시하고 전체 결과를 반환하는 stub을 생성한다.
동적 쿼리 지원 방안 (별도 Phase):

1. **sqlc overrides**: 쿼리 SQL에 `sqlc.arg()`로 optional 파라미터 추가
2. **raw SQL builder**: `squirrel` 등의 쿼리 빌더로 동적 WHERE/ORDER BY 생성
3. **sqlc + pgx**: `sqlc.narg()`로 nullable 파라미터 + COALESCE 패턴
