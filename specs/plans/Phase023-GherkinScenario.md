# Phase 023: Gherkin 시나리오 SSOT + Hurl 2단계 생성

## 목표

`specs/<project>/scenario/*.feature`에 Gherkin으로 크로스 엔드포인트 시나리오와 불변식을 선언한다. fullend가 교차 검증하고, Hurl 비즈니스 테스트를 자동 생성한다.

```
현재:
  OpenAPI → smoke.hurl (1단계: 엔드포인트별 스모크)
  크로스 엔드포인트 시나리오 = 없음

목표:
  scenario/*.feature → scenario-*.hurl (2단계: 비즈니스 시나리오)
  fullend validate가 .feature ↔ OpenAPI 교차 검증
  fullend gen이 .feature → Hurl 자동 생성
```

---

## Gherkin 문법 (제약된 고정 패턴)

표준 Gherkin의 자유형 영어 대신, 기계 파싱 가능한 고정 패턴만 허용한다.

### 액션 스텝 (Given/When/And)

```
METHOD operationId {JSON} → result     # 요청 + 캡처
METHOD operationId {JSON}              # 요청만 (캡처 불필요)
METHOD operationId → result            # body 없는 요청 + 캡처
METHOD operationId                     # body 없는 요청만
```

- `METHOD`: `GET`, `POST`, `PUT`, `DELETE`
- `operationId`: OpenAPI operationId (PascalCase)
- `{JSON}`: 요청 파라미터 (path param + body 통합). 변수 참조 가능
- `→ result`: 응답 캡처 변수명

### 어설션 스텝 (Then/And)

```
status == CODE                         # HTTP 상태 코드
response.field exists                  # 필드 존재
response.field == value                # 값 일치
response.array contains var.Field      # 배열에 포함
response.array excludes var.Field      # 배열에 미포함
response.array count > N               # 배열 크기
```

### JSON 내 변수 참조

```json
{"CourseID": course.ID, "Title": "Go 101", "Price": 10000}
```

- 따옴표 없는 `var.Field` → 이전 스텝에서 캡처한 변수 참조
- 따옴표 있는 값 → 리터럴

### 인증 규약

`→ token`으로 캡처하면 후속 요청에 `Authorization: Bearer {{token.AccessToken}}`이 자동 주입된다. security가 있는 operationId에만 적용.

### 태그

| 태그 | 의미 | Hurl 출력 |
|---|---|---|
| `@scenario` | 비즈니스 시나리오 | `scenario-{feature}.hurl` |
| `@invariant` | 불변식 검증 | `invariant-{feature}.hurl` |

---

## 예시: dummy-lesson

### `specs/dummy-lesson/scenario/course-lifecycle.feature`

```gherkin
@scenario
Feature: Instructor creates and publishes a course

  Scenario: Full course lifecycle
    Given POST Register {"Email": "inst@test.com", "Password": "Pass1234!", "Name": "Instructor"} → user
    And POST Login {"Email": "inst@test.com", "Password": "Pass1234!"} → token
    When POST CreateCourse {"Title": "Go 101", "Category": "dev", "Level": "beginner", "Price": 10000} → course
    And POST CreateLesson {"CourseID": course.ID, "Title": "Intro", "VideoURL": "https://example.com/v1", "SortOrder": 1} → lesson
    And PUT PublishCourse {"CourseID": course.ID}
    Then GET ListCourses → courses
    And response.courses contains course.ID
    And status == 200
```

### `specs/dummy-lesson/scenario/student-enrollment.feature`

```gherkin
@scenario
Feature: Student enrolls in a published course

  Background:
    Given POST Register {"Email": "inst@test.com", "Password": "Pass1234!", "Name": "Inst"} → instructor
    And POST Login {"Email": "inst@test.com", "Password": "Pass1234!"} → token
    And POST CreateCourse {"Title": "Go 101", "Category": "dev", "Level": "beginner", "Price": 10000} → course
    And PUT PublishCourse {"CourseID": course.ID}

  Scenario: Successful enrollment
    Given POST Register {"Email": "student@test.com", "Password": "Pass1234!", "Name": "Student"} → student
    And POST Login {"Email": "student@test.com", "Password": "Pass1234!"} → token
    When POST EnrollCourse {"CourseID": course.ID, "PaymentMethod": "card"} → enrollment
    Then status == 200
    And response.enrollment exists
    And response.payment exists
    And GET ListMyEnrollments → myEnrollments
    And response.enrollments contains enrollment.ID
```

### `specs/dummy-lesson/scenario/negative-auth.feature`

```gherkin
@scenario
Feature: Unauthorized access is denied

  Scenario: Enroll without auth
    When POST EnrollCourse {"CourseID": 1, "PaymentMethod": "card"}
    Then status == 401

  Scenario: Update course by non-owner
    Given POST Register {"Email": "other@test.com", "Password": "Pass1234!", "Name": "Other"} → user
    And POST Login {"Email": "other@test.com", "Password": "Pass1234!"} → token
    When PUT UpdateCourse {"CourseID": 1, "Title": "Hacked", "Category": "x", "Level": "x", "Price": 0}
    Then status == 403
```

### `specs/dummy-lesson/scenario/course-deletion.feature`

```gherkin
@invariant
Feature: Deleted course disappears from listing

  Scenario: Course excluded after deletion
    Given POST Register {"Email": "inst@test.com", "Password": "Pass1234!", "Name": "Inst"} → user
    And POST Login {"Email": "inst@test.com", "Password": "Pass1234!"} → token
    And POST CreateCourse {"Title": "Temp", "Category": "dev", "Level": "beginner", "Price": 0} → course
    When DELETE DeleteCourse {"CourseID": course.ID}
    Then status == 200
    And GET ListCourses → listing
    And response.courses excludes course.ID
```

---

## Hurl 생성 예시

`scenario/course-lifecycle.feature` → `tests/scenario-course-lifecycle.hurl`:

```hurl
# Auto-generated from scenario/course-lifecycle.feature — do not edit.

# Register
POST {{host}}/register
Content-Type: application/json
{
  "Email": "inst@test.com",
  "Password": "Pass1234!",
  "Name": "Instructor"
}

HTTP 200
[Captures]
user_id: jsonpath "$.user.ID"

# Login
POST {{host}}/login
Content-Type: application/json
{
  "Email": "inst@test.com",
  "Password": "Pass1234!"
}

HTTP 200
[Captures]
token: jsonpath "$.token.AccessToken"

# CreateCourse
POST {{host}}/courses
Authorization: Bearer {{token}}
Content-Type: application/json
{
  "Title": "Go 101",
  "Category": "dev",
  "Level": "beginner",
  "Price": 10000
}

HTTP 200
[Captures]
course_id: jsonpath "$.course.ID"

# CreateLesson
POST {{host}}/courses/{{course_id}}/lessons
Authorization: Bearer {{token}}
Content-Type: application/json
{
  "Title": "Intro",
  "VideoURL": "https://example.com/v1",
  "SortOrder": 1
}

HTTP 200
[Captures]
lesson_id: jsonpath "$.lesson.ID"

# PublishCourse
PUT {{host}}/courses/{{course_id}}/publish
Authorization: Bearer {{token}}

HTTP 200

# ListCourses
GET {{host}}/courses

HTTP 200
[Asserts]
jsonpath "$.courses" isCollection
jsonpath "$.courses[*].ID" includes {{course_id}}
```

---

## 교차 검증 규칙

### Scenario ↔ OpenAPI

| 규칙 | 수준 |
|---|---|
| 스텝의 operationId → OpenAPI operationId 존재 | ERROR |
| 스텝의 METHOD → OpenAPI에서 해당 operationId의 HTTP method 일치 | ERROR |
| 스텝의 JSON 필드 → OpenAPI request schema에 필드 존재 | ERROR |
| 캡처 결과 참조 필드 → OpenAPI response schema에 필드 존재 | WARNING |
| security 있는 operationId 호출 시 token 캡처 선행 확인 | WARNING |

### Scenario ↔ States

| 규칙 | 수준 |
|---|---|
| 상태 전이 operationId가 시나리오에 있으면 stateDiagram 전이와 순서 일치 | WARNING |

---

## 구현

### 새 파일

| 파일 | 역할 |
|---|---|
| `artifacts/internal/scenario/parser.go` | .feature 파서 (고정 패턴 정규식) |
| `artifacts/internal/scenario/types.go` | Feature, Scenario, Step, Assertion 구조체 |
| `artifacts/internal/scenario/parser_test.go` | 파서 테스트 |
| `artifacts/internal/crosscheck/scenario.go` | Scenario ↔ OpenAPI/States 교차 검증 |
| `artifacts/internal/gluegen/hurl_scenario.go` | .feature → scenario-*.hurl / invariant-*.hurl 생성 |

### 수정 파일

| 파일 | 변경 |
|---|---|
| `artifacts/internal/orchestrator/detect.go` | `KindScenario` + `scenario/*.feature` 감지 |
| `artifacts/internal/orchestrator/validate.go` | scenario 검증 단계 추가, cross에 시나리오 전달 |
| `artifacts/internal/orchestrator/gen.go` | scenario-gen 코드젠 단계 추가 |
| `artifacts/internal/crosscheck/crosscheck.go` | `Scenarios` 필드 + `CheckScenarios` 호출 |

### 더미 데이터

| 파일 | 역할 |
|---|---|
| `specs/dummy-lesson/scenario/course-lifecycle.feature` | 강의 생성→공개 시나리오 |
| `specs/dummy-lesson/scenario/student-enrollment.feature` | 수강 등록 시나리오 |
| `specs/dummy-lesson/scenario/negative-auth.feature` | 인가 실패 네거티브 테스트 |
| `specs/dummy-lesson/scenario/course-deletion.feature` | 삭제 후 목록 제외 불변식 |

### 출력

```
<artifacts-dir>/
  tests/
    smoke.hurl                          # 1단계 (Phase019, 유지)
    scenario-course-lifecycle.hurl      # 2단계: @scenario
    scenario-student-enrollment.hurl
    scenario-negative-auth.hurl
    invariant-course-deletion.hurl      # 2단계: @invariant
```

### 핵심 파서

```go
// Step 정규식 패턴
var reActionStep = regexp.MustCompile(
    `^(Given|When|Then|And|But)\s+` +
    `(GET|POST|PUT|DELETE)\s+` +
    `(\w+)` +                              // operationId
    `(?:\s+(\{.*\}))?` +                   // optional JSON
    `(?:\s+→\s+(\w+))?$`,                 // optional capture
)

var reAssertStatus = regexp.MustCompile(
    `^(Then|And)\s+status\s*==\s*(\d+)$`,
)

var reAssertResponse = regexp.MustCompile(
    `^(Then|And)\s+response\.(\w+)\s+` +
    `(exists|==|contains|excludes|count)\s*(.*)$`,
)
```

### Hurl 생성 로직

```go
func generateScenarioHurl(feature *Feature, doc *openapi3.T, outDir string) error
```

1. Feature의 각 Scenario를 순차 Hurl 요청으로 변환
2. Background가 있으면 각 Scenario 앞에 삽입
3. 액션 스텝:
   - operationId → OpenAPI path + method 역매핑
   - JSON의 path param 필드 → URL 치환, 나머지 → request body
   - `→ result` → `[Captures]` (response의 첫 번째 객체 필드에서 ID 추출)
   - token 캡처 후 security 엔드포인트 → `Authorization: Bearer {{token}}`
4. 어설션 스텝:
   - `status == CODE` → `HTTP CODE`
   - `response.field exists` → `jsonpath "$.field" exists`
   - `response.array contains var.ID` → `jsonpath "$.array[*].ID" includes {{var_id}}`
   - `response.array excludes var.ID` → `jsonpath "$.array[*].ID" not includes {{var_id}}`

---

## 문서 업데이트

### CLAUDE.md

SSOT 테이블에 추가:

```
| 시나리오 | `<root>/scenario/*.feature` | Gherkin (고정 패턴) |
```

교차 검증 규칙에 `Scenario ↔ OpenAPI` 섹션 추가.

gen 단계에 `scenario-gen` 추가.

### manual-for-ai.md

디렉토리 구조에 `scenario/*.feature` 추가.

Gherkin 시나리오 문법 섹션 신설:
- 고정 패턴 설명
- 액션/어설션 스텝 문법
- 태그 (@scenario / @invariant)
- 변수 참조 규칙

SSOT 연결 맵에 Scenario 추가.

교차 검증 테이블에 Scenario 규칙 추가.

### README.md

SSOT 목록에 `scenario/*.feature` 추가.

Runtime Testing 섹션 업데이트:

```markdown
Generated tests include:
- **smoke.hurl** — OpenAPI endpoint smoke tests (auto-generated)
- **scenario-*.hurl** — Business scenario tests (from .feature files)
- **invariant-*.hurl** — Cross-endpoint invariant tests (from .feature files)
```

---

## 의존성

- **Phase019 완료** ✅ (Hurl 1단계, hurl_util.go 재사용)
- **Phase020 완료** ✅ (stateDiagram, 교차 검증 연동)
- **Phase021 완료 필요** — OPA Rego 인가 정책 (네거티브 auth 시나리오 실행에 필요)
- **kin-openapi**: 이미 사용 중
- **Gherkin 파서**: 자체 구현 (고정 패턴 정규식, 외부 라이브러리 불필요)

## 검증

```bash
# 1. .feature 파일 작성
cat specs/dummy-lesson/scenario/course-lifecycle.feature

# 2. fullend validate
fullend validate specs/dummy-lesson
# ✓ Scenario    4 features, 6 scenarios

# 3. fullend gen
fullend gen specs/dummy-lesson artifacts/dummy-lesson

# 4. 생성된 Hurl 파일 확인
ls artifacts/dummy-lesson/tests/
# smoke.hurl
# scenario-course-lifecycle.hurl
# scenario-student-enrollment.hurl
# scenario-negative-auth.hurl
# invariant-course-deletion.hurl

# 5. (서버 실행 후) 전체 테스트
hurl --test --variable host=http://localhost:8080 artifacts/dummy-lesson/tests/*.hurl
```
