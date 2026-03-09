# fullend — AI SSOT 통합 가이드

> 5개 SSOT(OpenAPI, SQL DDL, SSaC, STML, Terraform)를 한 프로젝트에 작성할 때의 규칙.
> OpenAPI/SQL DDL/Terraform 자체 문법은 설명하지 않는다. SSaC/STML 고유 문법과 SSOT 간 연결 규칙만 다룬다.

## 프로젝트 디렉토리 구조

```
<project-root>/
├── api/openapi.yaml              # OpenAPI 3.x (x- 확장 포함)
├── db/
│   ├── *.sql                     # DDL (CREATE TABLE, CREATE INDEX)
│   └── queries/*.sql             # sqlc 쿼리 (-- name: Method :cardinality)
├── service/*.go                  # SSaC 선언 (Go 주석 DSL)
├── model/*.go                    # Go interface (component, func 정의)
├── frontend/
│   ├── *.html                    # STML 선언 (HTML5 + data-*)
│   ├── *.custom.ts               # 프론트엔드 계산 함수 (선택)
│   └── components/*.tsx          # React 컴포넌트 래퍼 (선택)
└── terraform/*.tf                # HCL 인프라 선언
```

## SSaC — 서비스 로직 선언

### 문법

```go
// @sequence <type>        — 블록 시작
// @model <Model.Method>   — 리소스 모델.메서드
// @param <Name> <source> [-> column]  — source: request | currentUser | 변수명 | "리터럴". -> column: 명시적 DDL 컬럼 매핑
// @result <var> <Type>    — 결과 바인딩
// @message "msg"          — 커스텀 에러 메시지 (선택)
// @var <name>             — response에서 반환할 변수
// @action @resource @id   — authorize 전용 (3개 모두 필수)
// @component | @func      — call 전용 (택일 필수)
```

### 10가지 시퀀스 타입

| 타입 | 용도 | 필수 태그 |
|---|---|---|
| authorize | 권한 검사 | @action, @resource, @id |
| get | 단건/목록 조회 | @model, @result |
| guard nil | nil이면 에러 반환 | target 변수명 |
| guard exists | nil 아니면 에러 반환 | target 변수명 |
| post | 생성 | @model, @result |
| put | 수정 | @model |
| delete | 삭제 | @model |
| password | 비밀번호 검증 | @param 2개 (hash, plain) |
| call | 외부 컴포넌트/함수 호출 | @component 또는 @func |
| response | JSON 응답 반환 | (없음, @var는 선택) |

### 예시: 모든 시퀀스 타입 사용

```go
// @sequence authorize
// @action update
// @resource course
// @id CourseID
//
// @sequence get
// @model Course.FindByID
// @param CourseID request
// @result course Course
//
// @sequence guard nil course
// @message "강의를 찾을 수 없습니다"
//
// @sequence get
// @model Enrollment.FindByCourseAndUser
// @param CourseID request
// @param UserID currentUser
// @result existing Enrollment
//
// @sequence guard exists existing
// @message "이미 수강 중입니다"
//
// @sequence password
// @param user.PasswordHash
// @param Password request
//
// @sequence post
// @model Enrollment.Create
// @param CourseID request
// @param UserID currentUser
// @result enrollment Enrollment
//
// @sequence put
// @model Course.IncrementEnrollCount
// @param CourseID request
//
// @sequence call
// @component notification
// @param enrollment
//
// @sequence response json
// @var enrollment
func EnrollCourse(w http.ResponseWriter, r *http.Request) {}
```

### @param source 규칙

| source | 의미 | 코드젠 |
|---|---|---|
| `request` | HTTP request body/query | `r.FormValue("Name")` |
| `currentUser` | 인증된 사용자 정보 | `currentUser.Name` |
| 변수명 | 이전 시퀀스의 @result 변수 | 직접 참조 |
| `"리터럴"` | 하드코딩 문자열 | 그대로 사용 |

`-> column` 매핑: `@param PaymentMethod request -> method` — 자동 snake_case 변환 대신 명시적 DDL 컬럼 매핑.

### 함수명 = operationId

SSaC 함수명은 반드시 OpenAPI의 operationId와 일치해야 한다. 이것이 프론트엔드(STML)와 백엔드(SSaC)를 연결하는 키다.

```
OpenAPI: operationId: EnrollCourse
SSaC:    func EnrollCourse(...)
STML:    data-action="EnrollCourse"
```

## STML — UI 선언

### 핵심 data-* 속성 (8개)

| 속성 | 값 | 용도 | 위치 |
|---|---|---|---|
| `data-fetch` | operationId | GET 바인딩 | 컨테이너 요소 |
| `data-action` | operationId | POST/PUT/DELETE 바인딩 | form/button 요소 |
| `data-field` | 필드명 | request body 필드 | data-action 내부 |
| `data-bind` | 필드명 (dot notation) | response 필드 출력 | data-fetch 내부 |
| `data-param-*` | `route.ParamName` | path/query 파라미터 | data-fetch 또는 data-action 요소 |
| `data-each` | 배열 필드명 | 목록 반복 | data-fetch 내부 |
| `data-state` | 조건식 | 조건부 렌더링 | 어디서든 |
| `data-component` | 컴포넌트명 | React 컴포넌트 위임 | 어디서든 |

### 인프라 data-* 속성 (3개)

| 속성 | 값 | 필요 조건 |
|---|---|---|
| `data-paginate` | (값 없음, boolean) | OpenAPI에 x-pagination 필요 |
| `data-sort` | `column` 또는 `column:desc` | OpenAPI에 x-sort 필요 |
| `data-filter` | `col1,col2` | OpenAPI에 x-filter 필요 |

### data-state 접미사 규칙

| 패턴 | 의미 | 코드젠 |
|---|---|---|
| `items.empty` | 배열이 비었을 때 | `{data.items?.length === 0 && ...}` |
| `items.loading` | 로딩 중 | `{isLoading && ...}` |
| `items.error` | 에러 발생 | `{isError && ...}` |
| `canEdit` | boolean 필드 | `{data.canEdit && ...}` |

### custom.ts 규칙

data-bind가 OpenAPI response에 없는 필드를 참조할 때, `<page>.custom.ts`에 같은 이름의 함수를 export하면 검증 통과.

```ts
// login-page.custom.ts
export function formattedDate(data) {
  return new Date(data.CreatedAt).toLocaleDateString()
}
```

### 예시: 복합 페이지

```html
<main>
  <section data-fetch="ListCourses" data-paginate data-sort="created_at:desc" data-filter="category,level">
    <ul data-each="courses">
      <li>
        <h3 data-bind="title"></h3>
        <span data-bind="price"></span>
        <div data-component="RatingStars" data-bind="averageRating"></div>
      </li>
    </ul>
    <p data-state="courses.empty">등록된 강의가 없습니다</p>
    <div data-state="courses.loading">로딩 중...</div>
  </section>

  <form data-action="CreateCourse">
    <input data-field="Title" placeholder="강의 제목" />
    <input data-field="Price" type="number" placeholder="가격" />
    <select data-field="Category">
      <option value="dev">개발</option>
      <option value="design">디자인</option>
    </select>
    <button type="submit">강의 등록</button>
  </form>
</main>
```

## OpenAPI x- 확장

OpenAPI 엔드포인트에 인프라 파라미터를 선언한다. SSaC spec에는 비즈니스 파라미터만 선언하고, 인프라 파라미터는 x-에만 선언한다.

```yaml
/courses:
  get:
    operationId: ListCourses
    x-pagination:
      style: offset           # offset | cursor
      defaultLimit: 20
      maxLimit: 100
    x-sort:
      allowed: [created_at, price, rating]
      default: created_at
      direction: desc          # asc | desc
    x-filter:
      allowed: [category, level, instructor_id]
    x-include:
      allowed: [instructor_id:users.id]   # FK컬럼:참조테이블.참조컬럼
```

### x-pagination

| 필드 | 타입 | 설명 |
|---|---|---|
| `style` | string | `offset` (Limit/Offset) 또는 `cursor` (커서 기반) |
| `defaultLimit` | int | 기본 페이지 크기 |
| `maxLimit` | int | 최대 페이지 크기 |

### x-sort

| 필드 | 타입 | 설명 |
|---|---|---|
| `allowed` | string[] | 정렬 가능 컬럼 (snake_case) |
| `default` | string | 기본 정렬 컬럼 |
| `direction` | string | `asc` 또는 `desc` |

### x-filter

| 필드 | 타입 | 설명 |
|---|---|---|
| `allowed` | string[] | 필터 가능 컬럼 (snake_case) |

### x-include

| 필드 | 타입 | 설명 |
|---|---|---|
| `allowed` | string[] | 정방향 FK include. 형식: `FK컬럼:참조테이블.참조컬럼` |

문법 (단일 형식만 허용):
- `instructor_id:users.id` — courses.instructor_id → users.id FK 관계로 User를 include
- 런타임 include 이름: FK 컬럼에서 `_id` 제거 (`instructor_id` → `instructor`)
- 역방향 FK (1:N)는 지원하지 않음 — 별도 엔드포인트로 처리

### x- 확장의 코드젠 영향

- SSaC: x- 있는 operation의 모델 메서드에 `opts QueryOpts` 파라미터 자동 추가
- SSaC: `:many` + x-pagination → 반환 타입 `([]T, int, error)` (total count 포함)
- STML: `data-paginate` → `useState(page, limit)` + prev/next 버튼 생성
- STML: `data-sort` → `useState(sortBy, sortDir)` + 토글 버튼 생성
- STML: `data-filter` → `useState(filters)` + 필터 입력 생성

## sqlc 쿼리 규칙

```sql
-- name: FindByID :one
SELECT * FROM courses WHERE id = $1;

-- name: List :many
SELECT * FROM courses ORDER BY created_at DESC;

-- name: Create :one
INSERT INTO courses (title, price, instructor_id)
VALUES ($1, $2, $3) RETURNING *;

-- name: SoftDelete :exec
UPDATE courses SET deleted_at = NOW() WHERE id = $1;
```

| 카디널리티 | SSaC @result 타입 | 코드젠 반환 |
|---|---|---|
| `:one` | `*Type` | `(*Course, error)` |
| `:many` | `[]Type` | `([]Course, error)` |
| `:exec` | (없음) | `error` |

모델명은 sqlc 쿼리 파일명에서 파생: `courses.sql` → `Course`
단수화 규칙: `ies`→`y`, `sses`→`ss`, `xes`→`x`, 나머지 `s` 제거

## model/*.go 규칙

SSaC `@component`와 `@func`의 참조 대상을 정의한다.

```go
// model/notification.go
package model

// NotificationService는 알림 컴포넌트다.
type NotificationService interface {
    Send(userID int64, message string) error
}
```

- `type XxxInterface interface` → `@component xxx`로 참조 가능
- `func Xxx(...)` → `@func xxx`로 참조 가능
- `// @dto` 주석이 달린 struct → DDL 테이블 매칭 건너뜀 (Token, Refund 같은 순수 DTO용)

## 5-SSOT 연결 맵

```
         OpenAPI (operationId)
           ↕               ↕
    SSaC (함수명)      STML (data-fetch/action)
      ↕
  DDL (테이블/컬럼)
      ↕
  sqlc 쿼리 (모델.메서드)
```

### 이름 매칭 규칙

| 소스 | 대상 | 매칭 |
|---|---|---|
| SSaC 함수명 | OpenAPI operationId | 동일 (PascalCase) |
| STML data-fetch/action | OpenAPI operationId | 동일 (PascalCase) |
| SSaC @model Model | DDL 테이블명 | PascalCase → snake_case + 복수형 (`Course` → `courses`) |
| SSaC @model .Method | sqlc 쿼리 `-- name:` | 동일 (`FindByID` = `FindByID`) |
| x-sort/filter allowed | DDL 컬럼명 | snake_case 동일 |
| x-include allowed | DDL FK 관계 | `FK컬럼:참조테이블.참조컬럼` → DDL FK 매핑 |

## fullend 교차 검증 규칙

개별 도구(ssac validate, stml validate)가 각자 검증한 후, fullend가 추가로 잡는 계층 간 불일치:

| 규칙 | 검증 내용 | 수준 |
|---|---|---|
| x-sort ↔ DDL | 컬럼이 테이블에 존재하는가 | ERROR |
| x-sort ↔ DDL index | 해당 컬럼에 인덱스가 있는가 | WARNING |
| x-filter ↔ DDL | 컬럼이 테이블에 존재하는가 | ERROR |
| x-include ↔ DDL FK | FK 관계로 연결된 테이블인가 | WARNING |
| SSaC @result ↔ DDL | 결과 타입에 대응하는 테이블이 있는가 | WARNING |
| SSaC @param ↔ DDL | 파라미터에 대응하는 컬럼이 있는가 | WARNING |
| SSaC 함수명 → operationId | SSaC 함수에 대응하는 operationId가 있는가 | ERROR |
| operationId → SSaC 함수명 | operationId에 대응하는 SSaC 함수가 있는가 | WARNING |

## fullend CLI

```bash
fullend validate <specs-dir>                 # 개별 검증 + 교차 검증
fullend gen <specs-dir> <artifacts-dir>      # validate → 코드젠
fullend status <specs-dir>                   # SSOT 현황 요약
```
