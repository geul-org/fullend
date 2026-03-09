✅ 완료

# Phase 014: sqlc 쿼리 접두사 + 생성 코드 컴파일 달성

## 목표

1. sqlc 쿼리 이름 충돌 해결 (모델명 접두사)
2. 수정지시서006 중 fullend 범위(6번, 7번) 처리
3. 최종 목표: `fullend gen` → `go build ./...` 성공

## 배경: 수정지시서006 실행결과

ssac에서 1~5번 완료됨. **6번, 7번은 fullend 범위**:

| # | 내용 | 담당 | 상태 |
|---|---|---|---|
| 1 | FindByID에 opts 잘못 전달 | ssac | ✅ `isListMethod()` 도입 |
| 2 | ListLessons에 opts 누락 | ssac | ✅ 이슈 1과 동일 수정 |
| 3 | 리터럴 파라미터 이름 잘못됨 | ssac | ✅ DDL ColumnOrder + resolveLiteralParamName |
| 4 | total이 response에 미포함 | ssac | ✅ funcHasTotal 플래그 |
| 5 | sqlc 쿼리 이름 접두사 | ssac | ✅ `stripModelPrefix()` |
| 6 | 서비스 파일 패키지 불일치 | **fullend** | 미완 |
| 7 | server.go model 타입 미한정 | **fullend** | 미완 |

---

## Step 1: dummy-lesson 쿼리 파일 접두사 적용 (SSOT 수정)

```
specs/dummy-lesson/db/queries/course.sql
  FindByID → CourseFindByID
  List → CourseList
  Create → CourseCreate
  Update → CourseUpdate
  Publish → CoursePublish
  Delete → CourseDelete

specs/dummy-lesson/db/queries/enrollments.sql
  FindByID → EnrollmentFindByID
  FindByCourseAndUser → EnrollmentFindByCourseAndUser
  ListByUser → EnrollmentListByUser
  Create → EnrollmentCreate

specs/dummy-lesson/db/queries/lessons.sql
  FindByID → LessonFindByID
  ListByCourse → LessonListByCourse
  Create → LessonCreate
  Update → LessonUpdate
  Delete → LessonDelete

specs/dummy-lesson/db/queries/payments.sql
  FindByID → PaymentFindByID
  ListByUser → PaymentListByUser
  Create → PaymentCreate
  UpdateStatus → PaymentUpdateStatus

specs/dummy-lesson/db/queries/reviews.sql
  FindByID → ReviewFindByID
  FindByCourseAndUser → ReviewFindByCourseAndUser
  ListByCourse → ReviewListByCourse
  Create → ReviewCreate
  Delete → ReviewDelete

specs/dummy-lesson/db/queries/users.sql
  FindByID → UserFindByID
  FindByEmail → UserFindByEmail
  Create → UserCreate
```

## Step 2: model_impl.go 쿼리 매칭 수정

현재 `parseQueryFiles()`는 `-- name: FindByID`로 매칭.
접두사 적용 후 `-- name: CourseFindByID`이므로 매칭 로직 변경:

- 현재: 쿼리 이름 == 메서드 이름 (`FindByID`)
- 수정: 쿼리 이름 == 모델명 + 메서드 이름 (`CourseFindByID`)

접두사 없는 기존 쿼리도 fallback으로 동작해야 한다 (ssac의 `stripModelPrefix()`와 동일 전략).

## Step 3: 서비스 파일 출력 경로 수정 (수정지시서006 #6)

### 문제

ssac이 서비스 파일을 `package internal`로 생성하지만, fullend glue-gen이 이를 `internal/service/` 디렉토리에 배치한다. Go에서 디렉토리 ≠ 패키지이면 컴파일 에러.

### 수정

ssac은 `package internal`을 생성하므로, fullend가 서비스 파일을 `internal/` 에 직접 배치해야 한다.

확인할 곳: ssac generator의 출력 경로 결정 로직 또는 fullend의 ssac 호출부에서 outDir 지정.

## Step 4: server.go model 타입 한정자 (수정지시서006 #7)

### 문제

glue-gen이 생성하는 `server.go`:
```go
package internal
type Server struct {
    courseModel CourseModel  // ← undefined
}
```

### 수정

```go
package internal

import "dummy-lesson/backend/internal/model"

type Server struct {
    courseModel model.CourseModel
}
```

glue-gen의 `generateServer()` (또는 해당 함수)에서:
- model 패키지 import 추가
- 필드 타입에 `model.` 접두사

## Step 5: 재생성 + 검증

```bash
# sqlc 컴파일
cd specs/dummy-lesson && sqlc compile  # → 에러 없음

# fullend 재생성
fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/

# 백엔드 빌드
cd artifacts/dummy-lesson/backend && go build ./...  # → 에러 없음
```

---

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `specs/dummy-lesson/db/queries/*.sql` | 쿼리 이름에 모델명 접두사 추가 |
| `artifacts/internal/gluegen/model_impl.go` | `parseQueryFiles()` 매칭: 모델명+메서드명, fallback 지원 |
| `artifacts/internal/gluegen/server_go.go` 또는 해당 파일 | server.go 생성 시 `model.` import + 타입 한정자 |
| ssac 호출부 또는 gluegen | 서비스 파일 출력 경로 `internal/` 직접 배치 |

## 의존성

- **Phase013 완료** ✅
- **ssac 수정지시서006 적용** ✅ (1~5번 완료, 6~7번은 본 Phase에서 처리)

## 검증

```bash
# 1. sqlc compile 성공
cd specs/dummy-lesson && sqlc compile

# 2. fullend gen 성공
fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/

# 3. go build 성공
cd artifacts/dummy-lesson/backend && go build ./...

# 4. 확인 사항:
#   - 서비스 파일이 internal/ 에 직접 위치 (internal/service/ 아님)
#   - server.go에 import "xxx/internal/model" + model.CourseModel 사용
#   - FindByID에 opts 없음, List에 opts 있음
#   - response에 total 포함
#   - UserModel.Create 4번째 파라미터가 role
#   - sqlc compile 에러 없음
```
