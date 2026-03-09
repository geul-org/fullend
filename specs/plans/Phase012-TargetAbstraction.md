# Phase 012: Target 추상화 통합 + glue-gen 버그 수정

## 목표

ssac 수정지시서004, stml 수정지시서002 완료에 따라 fullend 측 코드를 업데이트한다.
더불어 dummy-lesson 생성 결과에서 발견된 fullend glue-gen 버그 3건을 수정한다.

1. `GenerateWith(target, ...)` 호출로 전환하여 Target 슬롯 활용
2. fullend orchestrator에 `TargetProfile` 도입 — 백엔드/프론트엔드 Target 쌍 관리
3. 현재는 Go + React 고정, 구조만 확장 가능하게 준비
4. App.tsx — 존재하지 않는 페이지 import 방지
5. server.go — component 인터페이스 stub 생성
6. api.ts — stml 호출 규약과 일치하도록 object 파라미터 방식으로 변경

## 전제 (완료된 수정지시서)

- **ssac 004**: `Target` 인터페이스, `GoTarget`, `GenerateWith()`, 기존 API 하위 호환
- **stml 002**: `Target` 인터페이스, `ReactTarget`, `GenerateWith()`, 기존 API 하위 호환

## 전제 (별도 수정지시서 — Phase012와 병행)

- **ssac 005**: Generate()와 GenerateModelInterfaces() 시그니처 불일치 수정
- **stml 003**: Phase 5 infra 렌더링에서 변수명 불일치 수정

---

## 1. TargetProfile 도입

### 현재

gen.go에서 ssac/stml을 직접 호출한다. 언어/프레임워크 선택 개념이 없다.

```go
ssacgenerator.Generate(funcs, serviceOutDir, st)
stmlgenerator.Generate(pages, specsDir, outDir, opts)
```

### 변경

Target 쌍을 묶는 `TargetProfile`을 정의한다.

```go
// orchestrator/target_profile.go  ★ NEW
package orchestrator

import (
    ssacgenerator "github.com/geul-org/ssac/generator"
    stmlgenerator "github.com/geul-org/stml/generator"
)

// TargetProfile defines the backend + frontend code generation targets.
type TargetProfile struct {
    Backend  ssacgenerator.Target
    Frontend stmlgenerator.Target
}

// DefaultProfile returns Go backend + React frontend.
func DefaultProfile() *TargetProfile {
    return &TargetProfile{
        Backend:  ssacgenerator.DefaultTarget(),  // GoTarget
        Frontend: stmlgenerator.DefaultTarget(),  // ReactTarget
    }
}
```

### 구현

| 파일 | 변경 |
|---|---|
| `orchestrator/target_profile.go` | ★ NEW: TargetProfile 정의 + DefaultProfile() |

---

## 2. Gen() 함수에 TargetProfile 전달

### 현재

```go
func Gen(specsDir, artifactsDir string) (*reporter.Report, bool)
```

### 변경

```go
// 기존 시그니처 유지 (하위 호환)
func Gen(specsDir, artifactsDir string) (*reporter.Report, bool) {
    return GenWith(DefaultProfile(), specsDir, artifactsDir)
}

// GenWith는 지정된 TargetProfile로 코드를 생성한다.
func GenWith(profile *TargetProfile, specsDir, artifactsDir string) (*reporter.Report, bool) {
    // 기존 Gen() 본문, profile 전달
}
```

genSSaC, genSTML에 profile을 전달한다.

### 구현

| 파일 | 변경 |
|---|---|
| `orchestrator/gen.go` | Gen → GenWith 위임, genSSaC/genSTML에 profile 전달 |

---

## 3. SSaC GenerateWith() 전환

### 현재

```go
func genSSaC(specsDir, serviceDir, artifactsDir string) []reporter.StepResult {
    // ...
    ssacgenerator.Generate(funcs, serviceOutDir, st)
    ssacgenerator.GenerateModelInterfaces(funcs, st, modelOutDir)
}
```

### 변경

```go
func genSSaC(profile *TargetProfile, specsDir, serviceDir, artifactsDir string) []reporter.StepResult {
    // ...
    ssacgenerator.GenerateWith(profile.Backend, funcs, serviceOutDir, st)
    profile.Backend.GenerateModelInterfaces(funcs, st, modelOutDir)
}
```

### 구현

| 파일 | 변경 |
|---|---|
| `orchestrator/gen.go` `genSSaC()` | profile 파라미터 추가, GenerateWith 호출 |

---

## 4. STML GenerateWith() 전환

### 현재

```go
func genSTML(specsDir, frontendDir, artifactsDir string) (reporter.StepResult, map[string]string) {
    // ...
    result, err := stmlgenerator.Generate(pages, specsDir, outDir, stmlgenerator.GenerateOptions{
        APIImportPath: "../api",
        UseClient:     false,
    })
}
```

### 변경

```go
func genSTML(profile *TargetProfile, specsDir, frontendDir, artifactsDir string) (reporter.StepResult, map[string]string) {
    // ...
    result, err := stmlgenerator.GenerateWith(profile.Frontend, pages, specsDir, outDir, stmlgenerator.GenerateOptions{
        APIImportPath: "../api",
        UseClient:     false,
    })
}
```

### 구현

| 파일 | 변경 |
|---|---|
| `orchestrator/gen.go` `genSTML()` | profile 파라미터 추가, GenerateWith 호출 |

---

## 5. CLI에서 프로필 선택 (향후 확장 슬롯)

### 현재

CLI는 `fullend gen <specs-dir> <artifacts-dir>`만 지원한다.

### 변경 (Phase012 범위)

당장은 DefaultProfile 고정이다. 향후 `--target` 플래그를 추가할 수 있는 구조만 마련한다.

```go
// cmd/fullend/main.go — 변경 없음 (Gen() 호출 유지)
// Gen()이 내부적으로 DefaultProfile() 사용

// 향후 확장:
// fullend gen --backend=go --frontend=react specs/ artifacts/
// fullend gen --backend=java --frontend=vue specs/ artifacts/
```

CLI 플래그 추가는 Phase012 범위 밖이다. 지금은 `Gen()` → `GenWith(DefaultProfile(), ...)` 위임만 한다.

---

## 6. go.mod 업데이트

ssac, stml 최신 버전을 반영한다. `replace` 디렉티브는 로컬 개발이므로 유지.

```bash
cd ~/.clari/repos/fullend && go mod tidy
```

---

---

## 7. App.tsx — 존재하지 않는 페이지 import 방지

### 문제

`writeAppTSX()`가 OpenAPI의 모든 GET/POST 엔드포인트에서 라우트를 생성한다.
그러나 stml이 실제로 생성한 페이지와 대조하지 않으므로, 존재하지 않는 페이지를 import한다.

```tsx
// App.tsx가 import하지만 파일이 없는 것들
import GetCoursePage from './pages/get-course-page'       // ✗ 없음 (course-detail-page.tsx가 있음)
import ListLessonsPage from './pages/list-lessons-page'   // ✗ 없음
import ListReviewsPage from './pages/list-reviews-page'   // ✗ 없음
```

### 원인

App.tsx는 OpenAPI operationID에서 파생 (`GetCourse` → `GetCoursePage` → `get-course-page`).
stml 페이지는 STML 파일명에서 파생 (`course-detail.html` → `course-detail-page.tsx`).
이 두 명명 규칙이 일치하지 않으며, stml에 대응하는 페이지가 없는 엔드포인트도 있다.

### 수정

`writeAppTSX()`에 stml 생성 페이지 목록을 전달하여, 실제 존재하는 페이지만 라우트에 포함한다.

```go
// 변경 전
func writeAppTSX(srcDir string, doc *openapi3.T) error

// 변경 후
func writeAppTSX(srcDir string, doc *openapi3.T, stmlPages []string) error
```

stml 페이지 목록은 `genSTML()` 반환값에서 가져온다. stml이 없는 경우에도 OpenAPI 기반 라우트는 생성하되, 파일 존재 여부를 `pages/` 디렉토리 스캔으로 확인한다.

### 구현

| 파일 | 변경 |
|---|---|
| `gluegen/frontend.go` `writeAppTSX()` | stmlPages 파라미터 추가, 존재하는 페이지만 라우트 생성 |
| `gluegen/gluegen.go` `GlueInput` | `STMLPages []string` 필드 추가 |
| `orchestrator/gen.go` | stml 생성 결과에서 페이지 목록을 GlueInput에 전달 |

---

## 8. server.go — component 인터페이스 stub 생성

### 문제

`generateServerStruct()`가 component 필드를 `NotificationService` 타입으로 선언하지만, 해당 인터페이스를 생성하지 않는다.

```go
// server.go 현재 생성
type Server struct {
    notification NotificationService  // ← 타입 미정의
}
```

### 수정

component에 대한 인터페이스 stub을 server.go에 함께 생성한다.

```go
// server.go 변경 후
type NotificationService interface {
    Execute(args ...interface{}) error
}
```

SSaC spec의 `@component notification`이 실제로 어떤 메서드를 호출하는지 분석하여, 가능하면 정확한 시그니처를 생성한다.

### 구현

| 파일 | 변경 |
|---|---|
| `gluegen/server.go` `generateServerStruct()` | component 인터페이스 stub 생성 추가 |

---

## 9. api.ts — object 파라미터 방식으로 변경

### 문제

stml이 생성하는 API 호출:
```typescript
api.GetCourse({ courseid: CourseID, include: 'user,lesson' })
api.ListCourses({ page, limit, sortBy, sortDir, ...filters, include: 'user' })
api.EnrollCourse({ ...data, courseid: CourseID })
```

fullend가 생성하는 api.ts:
```typescript
async function getCourse(courseID: number | string) {  // positional
async function listCourses(params?: Record<string, string>) {  // no query string 전달
async function enrollCourse(courseID: number | string, body?: Record<string, unknown>) {  // positional + body
```

stml은 **항상 단일 object**를 전달하는데, api.ts는 **positional 파라미터**를 사용한다.
또한 GET 요청에서 query string을 URL에 붙이지 않는다.

### 수정

api.ts의 모든 함수를 단일 object 파라미터 방식으로 변경한다.

```typescript
// 변경 후
async function getCourse(params: { courseID: number | string, include?: string }) {
  const query = new URLSearchParams()
  if (params.include) query.set('include', params.include)
  const qs = query.toString()
  const res = await fetch(`${BASE}/courses/${params.courseID}${qs ? '?' + qs : ''}`)
  return res.json()
}

async function listCourses(params?: {
  page?: number, limit?: number, sortBy?: string, sortDir?: string,
  [key: string]: any
}) {
  const query = new URLSearchParams()
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v != null) query.set(k, String(v))
    }
  }
  const qs = query.toString()
  const res = await fetch(`${BASE}/courses${qs ? '?' + qs : ''}`)
  return res.json()
}
```

**핵심 변경:**
1. 모든 함수가 단일 object 파라미터 수용
2. path 파라미터를 object에서 추출하여 URL에 삽입
3. 나머지 필드를 query string으로 전달 (GET) 또는 body로 전달 (POST/PUT/DELETE)

### 구현

| 파일 | 변경 |
|---|---|
| `gluegen/frontend.go` `writeAPIClient()` | object 파라미터 방식으로 재작성 |

---

## 변경 파일 목록

| 파일 | 변경 유형 |
|---|---|
| `orchestrator/target_profile.go` | ★ NEW: TargetProfile + DefaultProfile() |
| `orchestrator/gen.go` | 수정: Gen → GenWith 위임, genSSaC/genSTML에 profile 전달, stmlPages 전달 |
| `gluegen/gluegen.go` | 수정: GlueInput에 STMLPages 필드 추가 |
| `gluegen/frontend.go` | 수정: writeAppTSX 페이지 필터링, writeAPIClient object 파라미터 |
| `gluegen/server.go` | 수정: component 인터페이스 stub 생성 |
| `go.mod` / `go.sum` | 수정: go mod tidy (ssac/stml 최신 반영) |

## 변경하지 않는 파일

| 파일 | 이유 |
|---|---|
| `cmd/fullend/main.go` | Gen() 시그니처 불변이므로 CLI 수정 불필요 |
| `crosscheck/*` | 교차 검증은 SSOT 레벨이므로 Target과 무관 |

## 의존성

- ssac 수정지시서004 완료 (Target, GoTarget, GenerateWith)
- stml 수정지시서002 완료 (Target, ReactTarget, GenerateWith)
- ssac 수정지시서005 (Generate/GenerateModelInterfaces 시그니처 일치) — 병행 가능
- stml 수정지시서003 (변수명 불일치) — 병행 가능

## 검증 방법

1. `go build ./artifacts/cmd/... ./artifacts/internal/...` 성공
2. `fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/` 실행
3. 코드에서 `GenerateWith` 호출 확인
4. `TargetProfile` 타입이 `ssacgenerator.Target`, `stmlgenerator.Target` 을 필드로 보유
5. 기존 테스트 통과 (`go test ./...`)
6. **App.tsx에 존재하지 않는 페이지 import가 없는지 확인**
7. **server.go에 component 인터페이스 정의 포함**
8. **api.ts의 모든 함수가 object 파라미터 수용**
9. **api.ts GET 요청에 query string 전달**

## 향후 확장 경로

Phase012 완료 후 추가 가능한 것들 (별도 Phase):

1. **CLI `--target` 플래그**: `DefaultProfile()` 대신 이름으로 프로필 선택
2. **glue-gen Target 추상화**: 현재 Go+React 하드코딩된 gluegen도 Target 기반으로 전환
3. **새 Target 구현**: JavaTarget, VueTarget 등을 ssac/stml에 추가하면 fullend에서 즉시 사용 가능
