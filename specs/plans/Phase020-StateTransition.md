# Phase 020: Mermaid stateDiagram → 상태 전이 검증

## 목표

리소스의 상태 전이를 Mermaid stateDiagram으로 선언하고, SSaC에서 `guard state`로 전이 가능 여부를 검증하는 구조를 도입한다.

```
현재:
  PublishCourse → Course.Publish 호출 (상태 전이가 암묵적)

목표:
  states/course.md에 stateDiagram 선언
  PublishCourse에 guard state → 전이 가능 여부 기계적 검증
  코드젠: if !CanTransition(course.Published, "PublishCourse") { return 409 }
```

---

## 설계

### 1. stateDiagram SSOT

`specs/<project>/states/*.md` 파일에 Mermaid stateDiagram을 선언한다.

```markdown
# CourseState

​```mermaid
stateDiagram-v2
    [*] --> draft
    draft --> published: PublishCourse
    published --> draft: UnpublishCourse
    published --> deleted: DeleteCourse
    draft --> deleted: DeleteCourse
​```
```

#### 규칙

- 파일명 = stateDiagram ID (예: `course.md` → `course`)
- 전이 레이블 = SSaC 함수명 = OpenAPI operationId (PascalCase)
- `[*]` → 초기 상태 (DDL DEFAULT 값과 일치해야 함)
- GitHub/IDE에서 Mermaid 렌더링으로 시각적 검증 가능
- 하나의 파일에 하나의 stateDiagram만 선언

#### dummy-lesson 예시

`specs/dummy-lesson/states/course.md`:

```markdown
# CourseState

​```mermaid
stateDiagram-v2
    [*] --> unpublished
    unpublished --> published: PublishCourse
    published --> deleted: DeleteCourse
    unpublished --> deleted: DeleteCourse
​```
```

courses 테이블의 `published BOOLEAN DEFAULT FALSE`에서:
- `unpublished` = `published = false` (초기 상태)
- `published` = `published = true`
- `deleted` = soft delete (`deleted_at IS NOT NULL`) 또는 hard delete

### 2. SSaC `guard state` 시퀀스 (11번째)

```go
// @sequence guard state course
// @param course.Published
```

#### 문법

```
// @sequence guard state {stateDiagramID}
// @param {entity}.{StatusField}
```

- `{stateDiagramID}`: `states/*.md` 파일명 (확장자 제외)
- `{entity}`: 이전 `@result`로 바인딩된 변수명
- `{StatusField}`: DDL 컬럼에 매핑되는 상태 필드
- **함수명이 전이 이벤트**: `PublishCourse` 함수 안의 `guard state course`는 현재 상태에서 `PublishCourse` 전이가 가능한지 검사

#### 예시: PublishCourse에 적용

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
// @sequence guard state course
// @param course.Published
//
// @sequence put
// @model Course.Publish
// @param CourseID request
//
// @sequence response json
func PublishCourse(w http.ResponseWriter, r *http.Request) {}
```

#### 예시: DeleteCourse에 적용

```go
// @sequence authorize
// @action delete
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
// @sequence guard state course
// @param course.Published
//
// @sequence delete
// @model Course.SoftDelete
// @param CourseID request
//
// @sequence response json
func DeleteCourse(w http.ResponseWriter, r *http.Request) {}
```

DeleteCourse는 `unpublished → deleted`와 `published → deleted` 둘 다 stateDiagram에 있으므로, 어떤 상태에서든 삭제 가능.

### 3. 코드젠

`guard state`가 생성하는 코드:

```go
// guard state: course (PublishCourse)
if !coursestate.CanTransition(course.Published, "PublishCourse") {
    http.Error(w, "invalid state transition", http.StatusConflict)
    return
}
```

상태 머신 패키지 (자동 생성):

```go
// states/coursestate/coursestate.go
package coursestate

// CanTransition checks if the given event is valid from the current state.
func CanTransition(published bool, event string) bool {
    current := stateFromPublished(published)
    _, ok := transitions[transitionKey{from: current, event: event}]
    return ok
}

func stateFromPublished(published bool) string {
    if published {
        return "published"
    }
    return "unpublished"
}

type transitionKey struct {
    from  string
    event string
}

var transitions = map[transitionKey]string{
    {"unpublished", "PublishCourse"}: "published",
    {"published", "DeleteCourse"}:   "deleted",
    {"unpublished", "DeleteCourse"}: "deleted",
}
```

### 4. 상태 필드 매핑

stateDiagram의 상태값 ↔ DDL 컬럼값 매핑은 필드 타입에 따라 결정:

| DDL 타입 | 상태 표현 | 매핑 |
|---|---|---|
| `BOOLEAN` | `true`/`false` | stateDiagram 상태명에서 추론 (published=true, unpublished=false) |
| `VARCHAR` (enum-like) | 문자열 | 상태명 = 컬럼값 (draft, published, archived) |
| `TIMESTAMPTZ` (soft delete) | `NULL`/`NOT NULL` | deleted = `deleted_at IS NOT NULL` |

BOOLEAN 매핑 규칙:
- 상태명이 필드명과 일치하면 `true` (예: `published` 상태 + `Published` 필드 → true)
- 접두사 `un` + 필드명이면 `false` (예: `unpublished` → false)

VARCHAR 매핑: 상태명을 그대로 컬럼값으로 사용.

---

## 교차 검증 규칙

### states ↔ SSaC

| 규칙 | 수준 |
|---|---|
| stateDiagram 전이 이벤트 → SSaC 함수 존재 | ERROR |
| SSaC에 guard state가 있으면 해당 stateDiagram 존재 | ERROR |
| stateDiagram에 전이가 있는 operationId에 guard state 없음 | WARNING |

### states ↔ DDL

| 규칙 | 수준 |
|---|---|
| guard state @param의 StatusField → DDL 컬럼 존재 | ERROR |
| 초기 상태([*] →) 값 ↔ DDL DEFAULT 값 일치 | WARNING |

### states ↔ OpenAPI

| 규칙 | 수준 |
|---|---|
| 전이 이벤트명 → OpenAPI operationId 존재 | ERROR |

---

## 구현

### 새 파일

| 파일 | 역할 |
|---|---|
| `artifacts/internal/statemachine/parser.go` | Mermaid stateDiagram 파서 (정규식 기반) |
| `artifacts/internal/statemachine/types.go` | StateDiagram, Transition 구조체 |
| `artifacts/internal/crosscheck/states.go` | states ↔ SSaC/DDL/OpenAPI 교차 검증 |
| `artifacts/internal/gluegen/stategen.go` | stateDiagram → Go 상태 머신 패키지 코드젠 |

### 수정 파일

| 파일 | 변경 |
|---|---|
| `artifacts/internal/orchestrator/detect.go` | states/ 디렉토리 감지 |
| `artifacts/internal/orchestrator/validate.go` | statemachine 검증 단계 추가 |
| `artifacts/internal/orchestrator/gen.go` | stategen 코드젠 단계 추가 |
| `artifacts/internal/gluegen/gluegen.go` | Generate()에 상태 머신 정보 전달 |

### SSaC 수정 (수정지시서)

| 파일 | 변경 |
|---|---|
| ssac `parser/` | `guard state` 시퀀스 파싱 (11번째 타입) |
| ssac `validator/` | `guard state` 검증 (stateDiagramID 참조 확인) |
| ssac `generator/` | `guard state` → CanTransition 코드젠 |

### 출력

```
<artifacts-dir>/
  states/
    coursestate/
      coursestate.go          # 상태 머신 패키지 (자동 생성)
```

### Mermaid 파서 핵심

```go
// parser.go

// ParseStateDiagram parses a Mermaid stateDiagram from markdown content.
func ParseStateDiagram(content string) (*StateDiagram, error)

// 추출 대상:
// 1. "[*] --> state" → 초기 상태
// 2. "stateA --> stateB: EventName" → 전이
// 3. 상태 목록 (전이에서 수집)
```

정규식:
```
초기 상태: \[\*\]\s*-->\s*(\w+)
전이:      (\w+)\s*-->\s*(\w+)\s*:\s*(\w+)
```

---

## Hurl 연동

stateDiagram이 있으면 Hurl 스모크 테스트에 전이 순서를 반영할 수 있다:
- `PublishCourse`는 `CreateCourse` 후에 실행 (draft → published 전이)
- 이미 Phase019의 CRUD 순서(POST → PUT)와 자연스럽게 호환

2단계(UseCase SSOT)에서는 네거티브 테스트 생성에 활용:
- 잘못된 상태에서 전이 시도 → 409 응답 확인

---

## 문서 업데이트

### CLAUDE.md

`교차 검증 규칙` 섹션에 추가:

```
### States ↔ SSaC
- stateDiagram 전이 이벤트 → SSaC 함수 존재
- guard state → 해당 stateDiagram 존재
### States ↔ DDL
- guard state 상태 필드 → DDL 컬럼 존재
```

`SSaC 참조` > `10가지 시퀀스 타입` → `11가지 시퀀스 타입` 갱신:

```
guard state — 상태 전이 가능 여부 검사
```

### manual-for-ai.md

SSaC 시퀀스 타입 테이블에 `guard state` 추가. 5-SSOT 연결 맵에 states 추가:

```
Mermaid stateDiagram → 상태 전이 검증
  - 전이 이벤트 = operationId = SSaC 함수명
  - guard state로 전이 가능 여부 검사
```

### README.md

Cross-Validation 섹션에 States 항목 추가.

---

## 의존성

- **SSaC 수정지시서**: `guard state` 파싱/검증/코드젠 (ssac 프로젝트에 요청)
- **kin-openapi**: 이미 사용 중
- **Mermaid 파서**: 자체 구현 (정규식, 외부 라이브러리 불필요)

## 검증

```bash
# 1. states/*.md 작성
cat specs/dummy-lesson/states/course.md

# 2. SSaC에 guard state 추가
cat specs/dummy-lesson/service/course/publish_course.go

# 3. fullend validate로 교차 검증
fullend validate specs/dummy-lesson

# 4. fullend gen으로 상태 머신 코드젠
fullend gen specs/dummy-lesson artifacts/dummy-lesson

# 5. 생성된 상태 머신 확인
cat artifacts/dummy-lesson/states/coursestate/coursestate.go

# 6. go build 확인
cd artifacts/dummy-lesson/backend && go build ./...
```
