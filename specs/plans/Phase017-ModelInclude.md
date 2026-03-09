✅ 완료 (Phase018에서 정방향 FK 전용으로 개선)

# Phase 017: Model Include 구현

## 목표

`?include=user,lesson` 쿼리 파라미터가 모델 구현체까지 전달되어 관계 리소스를 실제로 로딩하도록 한다. 서비스 레이어는 변경 없이 모델이 알아서 처리하고 반환한다.

```
현재:
  GET /courses?include=user
  서비스 → opts.Includes = ["user"]
  모델   → opts.Includes 무시, Course만 반환
  응답   → { "courses": [{ "id": 1, "instructor_id": 5 }] }

목표:
  GET /courses?include=user
  서비스 → opts.Includes = ["user"] (변경 없음)
  모델   → Includes 확인, User 추가 로딩, Course.Instructor에 첨부
  응답   → { "courses": [{ "id": 1, "instructor_id": 5, "instructor": { "name": "김교수" } }] }
```

---

## 핵심 설계

### 서비스 레이어 무변경

ssac이 생성하는 서비스 코드는 그대로:
```go
courses, total, err := h.CourseModel.List(opts)
json.Encode(map[string]interface{}{"courses": courses, "total": total})
```

모델 interface도 그대로:
```go
type CourseModel interface {
    List(opts QueryOpts) ([]Course, int, error)
}
```

**변경 범위는 fullend glue-gen이 생성하는 코드뿐이다.**

### struct에 Include 필드 추가

fullend가 생성하는 `types.go`의 struct에 FK 관계 기반 포인터 필드를 추가한다:

```go
// types.go (fullend 생성)
type Course struct {
    ID           int64     `json:"id"`
    InstructorID int64     `json:"instructor_id"`
    Title        string    `json:"title"`
    // ... DB 컬럼 필드

    // Include 필드 (scan 대상 아님, JSON omitempty)
    Instructor *User `json:"instructor,omitempty"`
}

type Enrollment struct {
    ID       int64 `json:"id"`
    UserID   int64 `json:"user_id"`
    CourseID int64 `json:"course_id"`

    Course *Course `json:"course,omitempty"`
}
```

`scanCourse()`는 DB 컬럼만 scan하므로 Include 필드는 nil 상태로 무시된다.

### Model 구현체에서 Include 로딩

```go
// model/course.go (fullend 생성)
func (m *courseModelImpl) List(opts QueryOpts) ([]Course, int, error) {
    // 기존: count + select 동적 SQL
    countSQL, countArgs := BuildCountQuery("courses", "published = TRUE", 0, opts)
    // ...
    selectSQL, selectArgs := BuildSelectQuery("courses", "published = TRUE", 0, opts)
    // ... scan items

    // Include 처리
    if containsStr(opts.Includes, "user") {
        m.includeUsers(items)
    }

    return items, total, nil
}

func (m *courseModelImpl) includeUsers(items []Course) {
    // 1. FK 값 수집 (중복 제거)
    ids := make(map[int64]bool)
    for _, c := range items {
        ids[c.InstructorID] = true
    }

    // 2. IN 쿼리로 일괄 로딩
    rows, err := m.db.QueryContext(ctx,
        "SELECT * FROM users WHERE id = ANY($1)", pq.Array(keys(ids)))
    // ... scan into map[int64]*User

    // 3. 각 item에 첨부
    for i := range items {
        items[i].Instructor = userMap[items[i].InstructorID]
    }
}
```

**N+1 문제 없음** — FK 값을 모아서 `WHERE id = ANY($1)` 단일 쿼리로 일괄 로딩.

---

## Include 매핑: x-include + DDL FK 자동 파생

### 정보 소스

| 소스 | 제공하는 정보 |
|---|---|
| OpenAPI `x-include` | include 이름 + 선택적 FK 컬럼 힌트 |
| DDL `REFERENCES` | FK 컬럼 → 대상 테이블 관계 |

### x-include 문법

```yaml
x-include:
  allowed: [user, lesson]              # 단일 FK — 테이블명만
  allowed: [instructor_id:user, lesson] # 다중 FK — FK컬럼:테이블 명시
```

- **단일 FK**: 해당 테이블을 참조하는 FK가 하나뿐이면 테이블명만 (`user`)
- **다중 FK**: 같은 테이블을 2개 이상 FK로 참조하면 `FK컬럼:테이블` 형식 (`instructor_id:user`)
- **역방향 FK**: 자기 테이블을 참조하는 외부 테이블도 테이블명만 (`lesson`)
- **모호성 에러**: 다중 FK인데 컬럼을 명시하지 않으면 glue-gen이 에러: `"ambiguous include 'user': courses has 2 FKs to users (instructor_id, reviewer_id). Use 'column:table' syntax"`

### 매핑 규칙

| x-include 값 | DDL FK 탐색 | 결과 |
|---|---|---|
| `user` | courses 테이블에서 `REFERENCES users(id)` → FK 1개 → `instructor_id` | `Instructor *User` 필드, FK=InstructorID |
| `instructor_id:user` | courses.instructor_id → `REFERENCES users(id)` 직접 지정 | `Instructor *User` 필드, FK=InstructorID |
| `lesson` | lessons 테이블에서 `REFERENCES courses(id)` → 역방향 | `Lessons []Lesson` 필드, 역FK 쿼리 |
| `course` | enrollments 테이블에서 `REFERENCES courses(id)` → `course_id` | `Course *Course` 필드, FK=CourseID |

### 정방향 vs 역방향 FK

```
정방향: courses.instructor_id → users.id
  x-include: "user" (또는 "instructor_id:user") → Course.Instructor *User
  쿼리: SELECT * FROM users WHERE id = ANY($1)

역방향: lessons.course_id → courses.id
  x-include: "lesson" → Course.Lessons []Lesson
  쿼리: SELECT * FROM lessons WHERE course_id = ANY($1)
```

### dummy-lesson 전체 매핑

| 엔드포인트 | x-include | FK 관계 | 생성 필드 | 쿼리 |
|---|---|---|---|---|
| `GET /courses` | `user` | `courses.instructor_id → users(id)` | `Instructor *User` | `SELECT * FROM users WHERE id = ANY($1)` |
| `GET /courses/{id}` | `user` | 동일 | `Instructor *User` | 동일 |
| `GET /courses/{id}` | `lesson` | `lessons.course_id → courses(id)` 역방향 | `Lessons []Lesson` | `SELECT * FROM lessons WHERE course_id = ANY($1)` |
| `GET /me/enrollments` | `course` | `enrollments.course_id → courses(id)` | `Course *Course` | `SELECT * FROM courses WHERE id = ANY($1)` |

---

## glue-gen 변경

### 1. DDL FK 파싱 확장 (`model_impl.go`)

현재 `parseDDLFiles()`가 컬럼명/타입만 파싱한다. FK 정보를 추가 추출한다:

```go
type ddlColumn struct {
    Name   string
    GoName string
    GoType string
    FKTable string // "" or "users" — REFERENCES 대상 테이블
}
```

파싱: `instructor_id BIGINT NOT NULL REFERENCES users(id)` → `FKTable = "users"`

### 2. Include 매핑 생성

x-include의 이름을 DDL FK에서 매핑하는 함수:

```go
type includeMapping struct {
    IncludeName string // "user"
    FieldName   string // "Instructor"
    FieldType   string // "*User" or "[]Lesson"
    IsReverse   bool   // false: 정방향 FK, true: 역방향 FK
    FKColumn    string // 정방향: "instructor_id", 역방향: "course_id"
    TargetTable string // "users" or "lessons"
    TargetModel string // "User" or "Lesson"
}

// includeSpec: "user", "instructor_id:user" 등 x-include 값을 파싱
// 단일 FK면 테이블명만으로 자동 매핑, 다중 FK면 컬럼 힌트 필수 (없으면 에러)
func resolveIncludes(model string, includeSpecs []string, tables map[string]*ddlTable) ([]includeMapping, error)
```

### 3. types.go에 Include 필드 추가

`generateTypesFile()`에서 FK 관계가 있는 컬럼에 대해 Include 포인터 필드를 추가:

```go
type Course struct {
    // DB 컬럼 (기존)
    ID           int64     `json:"id"`
    InstructorID int64     `json:"instructor_id"`
    ...

    // Include 필드 (신규)
    Instructor *User     `json:"instructor,omitempty"`
    Lessons    []Lesson  `json:"lessons,omitempty"`
}
```

### 4. Model 구현체에 Include 로딩 코드 추가

`generateMethodFromIface()`의 List 분기에서 include 로딩 코드를 생성:

```go
case isList:
    // 기존: count + select 동적 SQL
    ...

    // 신규: include 로딩
    for _, inc := range includeMap {
        b.WriteString(generateIncludeLoader(inc))
    }
```

### 5. include 헬퍼 함수 생성

`model/include_helpers.go`를 생성 (scan 함수 재사용):

```go
package model

func collectInt64s(ids map[int64]bool) []int64 { ... }
func containsStr(ss []string, s string) bool { ... }  // 이미 queryopts.go에 존재
```

---

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `gluegen/model_impl.go` | DDL FK 파싱 확장, List 메서드에 include 로딩 코드 생성, include 헬퍼 생성 |
| `gluegen/model_impl.go` | `generateTypesFile()`에 include 포인터 필드 추가 |
| `gluegen/gluegen.go` | Include 매핑 정보를 generateModelImpls에 전달 (OpenAPI x-include + DDL FK) |

## 의존성

- **Phase016 완료** ✅
- **ssac 수정 불필요** — 모델 interface, 서비스 코드 모두 변경 없음

## 검증

```bash
# 1. fullend gen
fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/

# 2. go build
cd artifacts/dummy-lesson/backend && go build ./...

# 3. 생성 코드 확인
# types.go: Course에 Instructor *User 필드 존재
# course.go: List 메서드에 includeUsers() 호출 존재

# 4. 수동 테스트
GET /courses?include=user
→ courses[].instructor 필드에 User 데이터 포함

GET /courses?include=invalid
→ ParseQueryOpts에서 allowed 체크, 무시됨

GET /courses
→ instructor 필드 없음 (omitempty)

GET /courses/1?include=user,lesson
→ instructor + lessons 둘 다 포함

GET /me/enrollments?include=course
→ enrollments[].course 필드에 Course 데이터 포함
```
