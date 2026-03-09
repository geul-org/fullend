# Phase 015: x-확장 런타임 구현 (QueryOpts 전체 체인)

## 목표

OpenAPI x-pagination/x-sort/x-filter/x-include가 **선언**만 되고 **런타임에 동작하지 않는** 상태를 해결한다.
프론트엔드(stml)는 이미 query param을 전달하고 있으므로, 백엔드 체인만 완성하면 된다.

```
현재:
  프론트 → api.ListCourses({ page:1, limit:20, sortBy:'price', filter:{category:'dev'} })
  서비스 → opts := QueryOpts{}    ← 빈 객체, HTTP param 무시
  모델   → q.List(ctx)            ← 정적 쿼리, opts 무시

목표:
  프론트 → api.ListCourses({ page:1, limit:20, sortBy:'price', filter:{category:'dev'} })
  서비스 → opts := parseQueryOpts(r, xConfig)   ← HTTP param → QueryOpts 바인딩
  모델   → dynamicList(db, opts)                ← 동적 SQL 실행
```

---

## 전체 체인 (3개 레이어)

```
Layer 1: HTTP request → QueryOpts      (ssac 코드젠)
Layer 2: QueryOpts → 동적 SQL          (fullend Model 구현체)
Layer 3: include → 관계 리소스 로딩     (fullend Model 구현체)
```

---

## Layer 1: HTTP → QueryOpts 바인딩 (ssac 수정지시서)

### 현재

ssac이 생성하는 코드:

```go
func (s *Server) ListCourses(w http.ResponseWriter, r *http.Request) {
    opts := QueryOpts{}  // ← 빈 객체
    courses, total, err := s.courseModel.List(opts)
}
```

### 목표

x-확장 정보를 기반으로 HTTP query param을 파싱하는 코드를 생성한다:

```go
func (s *Server) ListCourses(w http.ResponseWriter, r *http.Request) {
    opts := parseQueryOpts(r, QueryOptsConfig{
        Pagination: &PaginationConfig{
            Style:        "offset",
            DefaultLimit: 20,
            MaxLimit:     100,
        },
        Sort: &SortConfig{
            Allowed:   []string{"created_at", "price"},
            Default:   "created_at",
            Direction: "desc",
        },
        Filter: &FilterConfig{
            Allowed: []string{"category", "level"},
        },
        Include: &IncludeConfig{
            Allowed: []string{"user"},
        },
    })
    courses, total, err := s.courseModel.List(opts)
}
```

### parseQueryOpts 동작

| query param | QueryOpts 필드 | 기본값 | 검증 |
|---|---|---|---|
| `?limit=N` | Limit | defaultLimit | min(N, maxLimit) |
| `?offset=N` | Offset | 0 | ≥ 0 |
| `?cursor=xxx` | Cursor | "" | (cursor 방식일 때) |
| `?sortBy=col` | SortCol | x-sort.default | x-sort.allowed에 포함 |
| `?sortDir=asc` | SortDir | x-sort.direction | asc \| desc |
| `?category=dev` | Filters["category"] | — | x-filter.allowed에 포함 |
| `?include=user,lesson` | Includes | — | x-include.allowed에 포함 |

### 구현 방식

**방안 A: ssac가 인라인 생성** — 각 서비스 함수에 파싱 코드를 직접 생성
**방안 B: 공통 함수 + config** — `parseQueryOpts(r, config)` 함수를 한 번 생성하고, 서비스에서 config만 다르게 전달

**방안 B 채택.** 이유:
- 파싱 로직이 모든 List 엔드포인트에서 동일
- config만 x-확장에서 파생
- `parseQueryOpts`는 glue-gen이 생성하는 공통 유틸

### 담당

| 역할 | 담당 |
|---|---|
| `parseQueryOpts()` 함수 + Config 타입 생성 | fullend glue-gen |
| 서비스에서 `parseQueryOpts(r, config)` 호출 코드 생성 | ssac 수정지시서 007 |
| config 값은 SymbolTable의 x-확장에서 추출 | ssac generator |

---

## Layer 2: QueryOpts → 동적 SQL (Model 구현체)

### 현재 (Phase013 stub)

```go
func (m *courseModelImpl) List(opts QueryOpts) ([]Course, int, error) {
    items, err := m.q.List(context.Background())
    // TODO: apply opts
    return items, len(items), nil
}
```

sqlc의 `List()` 쿼리는 정적이다:
```sql
-- name: List :many
SELECT * FROM courses WHERE published = TRUE ORDER BY created_at DESC;
```

### 목표

List 메서드를 **동적 SQL**로 교체한다. sqlc 쿼리는 비-List 메서드(FindByID, Create, Update, Delete)에만 사용한다.

```go
func (m *courseModelImpl) List(opts QueryOpts) ([]Course, int, error) {
    // 1. COUNT 쿼리
    countSQL, countArgs := buildCountQuery("courses", "published = TRUE", opts)
    var total int
    m.db.QueryRow(context.Background(), countSQL, countArgs...).Scan(&total)

    // 2. SELECT 쿼리
    selectSQL, selectArgs := buildSelectQuery("courses", "published = TRUE", opts)
    rows, err := m.db.Query(context.Background(), selectSQL, selectArgs...)
    // ... scan rows into []Course
    return courses, total, nil
}
```

### 동적 SQL 빌더

`database/sql` 직접 사용. 외부 라이브러리(squirrel 등) 없이 간단한 빌더를 glue-gen이 생성한다:

```go
// querybuilder.go (glue-gen 생성)
package model

import "fmt"

// buildSelectQuery constructs a dynamic SELECT with pagination, sort, filter.
func buildSelectQuery(table, baseWhere string, opts QueryOpts) (string, []interface{}) {
    var args []interface{}
    argIdx := 1

    sql := fmt.Sprintf("SELECT * FROM %s WHERE %s", table, baseWhere)

    // Filter
    for col, val := range opts.Filters {
        sql += fmt.Sprintf(" AND %s = $%d", col, argIdx)
        args = append(args, val)
        argIdx++
    }

    // Sort
    if opts.SortCol != "" {
        dir := "ASC"
        if opts.SortDir == "desc" {
            dir = "DESC"
        }
        sql += fmt.Sprintf(" ORDER BY %s %s", opts.SortCol, dir)
    }

    // Pagination (offset)
    if opts.Limit > 0 {
        sql += fmt.Sprintf(" LIMIT $%d", argIdx)
        args = append(args, opts.Limit)
        argIdx++
    }
    if opts.Offset > 0 {
        sql += fmt.Sprintf(" OFFSET $%d", argIdx)
        args = append(args, opts.Offset)
        argIdx++
    }

    return sql, args
}

func buildCountQuery(table, baseWhere string, opts QueryOpts) (string, []interface{}) {
    var args []interface{}
    argIdx := 1

    sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, baseWhere)

    for col, val := range opts.Filters {
        sql += fmt.Sprintf(" AND %s = $%d", col, argIdx)
        args = append(args, val)
        argIdx++
    }

    return sql, args
}
```

### SQL 인젝션 방지

- **컬럼명 검증**: SortCol, Filter key는 x-확장의 `allowed` 목록과 대조. 허용된 값만 SQL에 삽입
- **값은 항상 $N 파라미터**: 사용자 입력은 절대 SQL 문자열에 직접 삽입하지 않음
- **검증 위치**: `parseQueryOpts()`에서 allowed 체크 완료 → Model에 도달하는 opts는 이미 안전

```go
// parseQueryOpts 내부
if !contains(config.Sort.Allowed, sortBy) {
    sortBy = config.Sort.Default // 허용 안 된 컬럼은 기본값으로 대체
}
```

### baseWhere 결정

sqlc 쿼리 SQL에서 WHERE 절을 추출한다:

| sqlc 쿼리 | baseWhere |
|---|---|
| `SELECT * FROM courses WHERE published = TRUE ORDER BY ...` | `published = TRUE` |
| `SELECT * FROM enrollments WHERE user_id = $1 ORDER BY ...` | `user_id = $1` |
| `SELECT * FROM reviews WHERE course_id = $1 ORDER BY ...` | `course_id = $1` |

glue-gen이 sqlc 쿼리 SQL을 파싱하여 baseWhere를 추출한다.

### Model 구현체 변경

Phase013 stub과 달리, List 메서드는 `*db.Queries` 대신 `*sql.DB`를 직접 사용한다:

```go
type courseModelImpl struct {
    q  *db.Queries   // FindByID, Create, Update, Delete 용
    db *sql.DB       // List 동적 쿼리 용
}

func NewCourseModel(q *db.Queries, conn *sql.DB) CourseModel {
    return &courseModelImpl{q: q, db: conn}
}
```

### Row 스캔

sqlc는 내부적으로 row scan 코드를 생성하지만, 동적 쿼리에서는 직접 scan해야 한다.
테이블 컬럼 순서는 DDL에서 파악 가능:

```go
func scanCourse(rows *sql.Rows) (Course, error) {
    var c Course
    err := rows.Scan(&c.ID, &c.InstructorID, &c.Title, &c.Description,
        &c.Category, &c.Level, &c.Price, &c.Published, &c.CreatedAt)
    return c, err
}
```

이 scan 함수도 glue-gen이 DDL 컬럼 정보로부터 생성한다.

---

## Layer 3: Include — 관계 리소스 로딩

### 전략

**N+1 분리 쿼리** 방식을 채택한다. JOIN은 sqlc 쿼리 구조를 과도하게 변경해야 하므로 Phase014 범위에서는 분리 쿼리로 한다.

```go
// include=user 처리 예시
func (m *courseModelImpl) List(opts QueryOpts) ([]Course, int, error) {
    courses, total := // ... 동적 SQL로 조회

    // Include 처리
    if contains(opts.Includes, "user") {
        userIDs := unique(mapField(courses, func(c Course) int64 { return c.InstructorID }))
        users, _ := m.loadUsers(userIDs)
        // courses에 user 정보 첨부
    }

    return courses, total, nil
}
```

### Include 반환 타입

현재 interface의 `List(opts) ([]Course, int, error)` 시그니처에서 Course struct에 include된 리소스를 어떻게 담을 것인가:

**방안 A**: Course struct에 포인터 필드 추가 (`User *User`, `Lessons []Lesson`)
**방안 B**: 별도 응답 타입 사용 (`CourseWithIncludes`)
**방안 C**: `map[string]interface{}` 래핑

**방안 A 채택.** 이유:
- sqlc struct에 `json:"-"` 태그로 optional 필드 추가 가능
- 프론트엔드가 `course.instructor.name`을 기대하므로 중첩 struct가 자연스러움

```go
// glue-gen이 생성하는 확장 타입
type CourseWithIncludes struct {
    Course
    Instructor *User     `json:"instructor,omitempty"`
    Reviews    []Review  `json:"reviews,omitempty"`
}
```

그러나 이렇게 하면 interface 반환 타입이 바뀐다. 이는 ssac model interface와 충돌한다.

**대안**: service 레이어에서 include 처리. Model은 순수 CRUD만 담당.

```go
// service/list_courses.go
courses, total, err := s.courseModel.List(opts)
// include는 service에서 직접 처리
response := map[string]interface{}{
    "courses": courses,
    "total":   total,
}
if contains(opts.Includes, "user") {
    // 별도 로딩
}
```

**Phase014에서는 service 레이어 include 방식을 채택한다.** Model interface 변경이 불필요하고, ssac 수정 범위가 작다.

---

## 변경 파일 목록

### fullend (직접 수정)

| 파일 | 변경 |
|---|---|
| `gluegen/queryopts.go` | ★ NEW: `parseQueryOpts()` + Config 타입 + `buildSelectQuery()` + `buildCountQuery()` 생성 |
| `gluegen/model_impl.go` | 수정: List 메서드를 동적 SQL로 교체, `*sql.DB` 필드 추가, scan 함수 생성 |
| `gluegen/main_go.go` | 수정: `NewXxxModel(queries, db)` 호출에 `*sql.DB` 전달 |
| `gluegen/gluegen.go` | 수정: DDL 컬럼 정보를 GlueInput에 추가 |

### ssac 수정지시서 007

| 변경 | 내용 |
|---|---|
| `opts := QueryOpts{}` → `opts := parseQueryOpts(r, config)` | x-확장이 있는 서비스에서 파싱 코드 생성 |
| config 값은 SymbolTable에서 추출 | x-pagination/sort/filter/include → Config 리터럴 |

---

## 생성 산출물 (dummy-lesson 기준)

| 파일 | 내용 |
|---|---|
| `backend/internal/model/querybuilder.go` | `parseQueryOpts()`, `buildSelectQuery()`, `buildCountQuery()` |
| `backend/internal/model/scan.go` | 모델별 row scan 함수 (`scanCourse`, `scanLesson`, ...) |
| `backend/internal/model/course.go` | List → 동적 SQL 사용 |
| `backend/internal/model/enrollment.go` | ListByUser → 동적 SQL |
| `backend/internal/model/review.go` | ListByCourse → 동적 SQL |
| `backend/internal/model/payment.go` | ListByUser → 동적 SQL |
| `backend/internal/service/list_courses.go` | `opts := parseQueryOpts(r, config)` (ssac 재생성) |

## 의존성

- **Phase013 완료** (Model 구현체 stub 존재)
- **ssac 수정지시서 007** (parseQueryOpts 호출 코드 생성) — 병행 가능

## 검증

1. `fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/` 재실행
2. `go build ./...` 성공
3. `list_courses.go`에서 `parseQueryOpts(r, ...)` 호출 확인
4. `model/course.go` List 메서드에 동적 SQL 확인
5. 수동 테스트: `GET /api/courses?limit=5&sortBy=price&sortDir=asc&category=dev`
   - 5건 반환, 가격순 정렬, category=dev 필터링 확인
6. 수동 테스트: `GET /api/courses?sortBy=INVALID_COL`
   - 기본값(created_at desc)으로 fallback 확인
7. SQL 인젝션 테스트: `GET /api/courses?sortBy=id;DROP TABLE courses`
   - allowed 체크에 의해 거부, 기본값 사용

## 향후 Phase

- **커서 기반 페이지네이션**: offset 방식 외 cursor 방식 구현 (x-pagination.style=cursor)
- **Include JOIN 최적화**: N+1 → 단일 JOIN 쿼리로 전환
- **Full-text search**: x-search 확장 추가
