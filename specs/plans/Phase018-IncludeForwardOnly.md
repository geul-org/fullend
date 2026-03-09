✅ 완료

# Phase 018: x-include 정방향 FK 전용 + 문법 통일

## 목표

x-include 문법을 `column:table.column` 단일 포맷으로 통일한다. 정방향 FK만 선언하고, 역방향 FK 지원을 제거한다.

```
현재 (Phase017):
  x-include:
    allowed: [user, lesson, instructor_id:user]
  → 3가지 형식 혼재, 역방향 FK 지원, 자동 매핑

목표:
  x-include:
    allowed: [instructor_id:users.id]
  → 단일 형식, 정방향 FK만, 명시적 선언
```

---

## 설계 근거

### 왜 정방향 FK만?

- **정방향 (N:1)**: 각 아이템에 관련 객체 1개 → 데이터 크기 예측 가능
- **역방향 (1:N)**: 각 아이템에 관련 객체 N개 → 데이터 폭발 위험
- 1:N 관계는 별도 엔드포인트로 처리: `GET /courses/{id}/lessons`

### 왜 단일 포맷?

x-include는 SSOT다. 모호한 자동 매핑 대신 명시적 선언이 원칙에 맞다.

- `user` → 어떤 FK 컬럼인지 모호
- `instructor_id:users.id` → `courses.instructor_id → users.id` 명확

---

## x-include 새 문법

```yaml
x-include:
  allowed:
    - instructor_id:users.id      # courses.instructor_id → users.id
    - course_id:courses.id        # enrollments.course_id → courses.id
```

### 포맷

```
<local_fk_column>:<target_table>.<target_column>
```

| 부분 | 설명 | 예시 |
|---|---|---|
| `local_fk_column` | 현재 테이블의 FK 컬럼 | `instructor_id` |
| `target_table` | 참조 대상 테이블 (DDL 테이블명, 복수형) | `users` |
| `target_column` | 참조 대상 컬럼 (보통 `id`) | `id` |

### 파생 규칙

| 소스 | 파생 | 예시 |
|---|---|---|
| `local_fk_column` | 런타임 include 이름 | `instructor_id` → `instructor` (`_id` 제거) |
| `local_fk_column` | struct 필드명 | `instructor_id` → `Instructor` (snake→Go, `_id` 제거) |
| `target_table` | 필드 타입 | `users` → `*User` (singularize) |

### HTTP 요청

```
GET /courses?include=instructor
GET /me/enrollments?include=course
```

include 파라미터 값 = FK 컬럼에서 `_id`를 뺀 이름.

---

## dummy-lesson 매핑

| 엔드포인트 | x-include | FK 관계 | 생성 필드 | 런타임 이름 |
|---|---|---|---|---|
| `GET /courses` | `instructor_id:users.id` | `courses.instructor_id → users.id` | `Instructor *User` | `instructor` |
| `GET /courses/{id}` | `instructor_id:users.id` | 동일 | `Instructor *User` | `instructor` |
| `GET /me/enrollments` | `course_id:courses.id` | `enrollments.course_id → courses.id` | `Course *Course` | `course` |

**제거**: `GET /courses/{id}`의 `lesson` include (역방향 FK)

---

## 변경 사항

### 1. gluegen/model_impl.go

#### includeMapping 구조체 — IsReverse 제거

```go
type includeMapping struct {
    IncludeName string // "instructor" — FK 컬럼에서 _id 제거
    FieldName   string // "Instructor"
    FieldType   string // "*User"
    FKColumn    string // "instructor_id"
    TargetTable string // "users"
    TargetModel string // "User"
}
```

#### resolveIncludes — 새 포맷 파싱, 역방향 제거

```go
func resolveIncludes(modelName string, includeSpecs []string, tables map[string]*ddlTable) ([]includeMapping, error)
```

파싱:
1. `instructor_id:users.id` → localColumn=`instructor_id`, targetTable=`users`, targetColumn=`id`
2. 현재 테이블에서 localColumn 찾기
3. FKTable이 targetTable과 일치하는지 검증
4. includeName = localColumn에서 `_id` 제거

에러:
- 포맷 불일치: `"invalid x-include format 'xxx': expected 'column:table.column'"`
- FK 컬럼 미존재: `"column xxx not found in table yyy"`
- FK 대상 불일치: `"column xxx does not reference yyy"`

#### generateIncludeHelper — 역방향 분기 제거

정방향 FK 로직만 유지 (현재 else 분기).

#### pluralize 함수 — 제거

역방향 FK에서만 사용되었으므로 불필요.

### 2. gluegen/gluegen.go

#### buildQueryOptsConfig — 런타임 이름 파생

```go
// "instructor_id:users.id" → "instructor"
colonIdx := strings.Index(spec, ":")
localCol := spec[:colonIdx]
runtimeName := strings.TrimSuffix(localCol, "_id")
```

### 3. specs/dummy-lesson/api/openapi.yaml

```yaml
# Before
x-include:
  allowed: [user]
x-include:
  allowed: [user, lesson]
x-include:
  allowed: [course]

# After
x-include:
  allowed: [instructor_id:users.id]
x-include:
  allowed: [instructor_id:users.id]        # lesson 제거
x-include:
  allowed: [course_id:courses.id]
```

### 4. 매뉴얼 업데이트

| 파일 | 변경 |
|---|---|
| `fullend/artifacts/manual-for-ai.md` | x-include 문법 → 단일 포맷, 역방향 제거 |
| `ssac/artifacts/manual-for-ai.md` | x-include 주석 업데이트 |
| `ssac/artifacts/manual-for-human.md` | x-include 섹션 업데이트 |

### 5. Phase017 계획서

역방향 FK 관련 내용 제거, Phase018에서 대체되었음을 명시.

---

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `gluegen/model_impl.go` | includeMapping 간소화, resolveIncludes 새 포맷, 역방향 제거, pluralize 제거 |
| `gluegen/gluegen.go` | buildQueryOptsConfig 런타임 이름 파생 방식 변경 |
| `specs/dummy-lesson/api/openapi.yaml` | x-include 값을 새 포맷으로 변경 |
| `fullend/artifacts/manual-for-ai.md` | x-include 문서 업데이트 |
| `ssac/artifacts/manual-for-ai.md` | x-include 문서 업데이트 |
| `ssac/artifacts/manual-for-human.md` | x-include 문서 업데이트 |

## 의존성

- **Phase017 완료** ✅

## 검증

```bash
# 1. fullend gen
fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/

# 2. 생성 코드 확인
# types.go: Course에 Instructor *User (Lessons 없음)
# types.go: Enrollment에 Course *Course
# course.go: List에 includeInstructor() (includeLesson 없음)
# enrollment.go: ListByUser에 includeCourse()

# 3. go build
cd artifacts/dummy-lesson/backend && go build ./internal/model/

# 4. 잘못된 포맷 에러 확인
# x-include: allowed: [user] → 에러: "invalid x-include format"
```
