# Phase 016: 도메인 폴더 구조 지원 (ssac 수정지시서007 후속) ✅ 완료

## 목표

ssac 수정지시서007(도메인 폴더 구조) 적용 후, fullend의 glue-gen과 orchestrator를 수정하여 도메인별 서비스 패키지를 올바르게 처리한다.

```
specs/service/             internal/service/
├── course/                ├── course/
│   ├── create_course.go   │   ├── create_course.go  ← package course
│   └── list_courses.go    │   └── list_courses.go
├── auth/                  ├── auth/
│   ├── login.go           │   ├── login.go           ← package auth
│   └── register.go        │   └── register.go
└── review/                └── review/
    └── create_review.go       └── create_review.go   ← package review
```

## ssac 수정지시서007 확인된 API

```go
// parser/types.go
type ServiceFunc struct {
    Name      string
    FileName  string
    Domain    string     // 도메인 폴더명 (e.g. "course"). 빈 문자열이면 루트.
    Sequences []Sequence
}
```

- `ParseDir`이 `filepath.WalkDir`로 재귀 탐색
- 상대 경로의 첫 번째 디렉토리 → `Domain` 파생: `service/course/create_course.go` → `Domain="course"`
- flat 파일(`service/login.go`) → `Domain=""`
- generator: `Domain != ""` → `outDir/{domain}/` 서브디렉토리, `package {domain}`
- generator: `Domain == ""` → `outDir/`, `package service` (기존 동작)

## 핵심 과제

도메인별 패키지로 분리되면 `Server` struct를 어디에 둘 것인가, 각 도메인의 핸들러를 어떻게 통합할 것인가가 핵심 문제다.

---

## 설계: 도메인별 Handler + 중앙 Server

### 구조

```
internal/service/
├── server.go              ← package service (중앙 Server + Handler)
├── auth.go                ← package service (CurrentUser 등 공통)
├── course/
│   ├── handler.go         ← package course (CourseHandler struct + 메서드)
│   ├── create_course.go
│   └── list_courses.go
├── auth/
│   ├── handler.go         ← package auth (AuthHandler struct + 메서드)
│   ├── login.go
│   └── register.go
└── review/
    ├── handler.go         ← package review (ReviewHandler struct + 메서드)
    ├── create_review.go
    └── list_reviews.go
```

### 도메인별 Handler struct

각 도메인 패키지에 자체 Handler struct를 생성한다:

```go
// internal/service/course/handler.go
package course

import "dummy-lesson/backend/internal/model"

type Handler struct {
    CourseModel model.CourseModel
    LessonModel model.LessonModel  // 이 도메인이 사용하는 모델만
}
```

```go
// internal/service/course/create_course.go
package course

func (h *Handler) CreateCourse(w http.ResponseWriter, r *http.Request) {
    // ...
    course, err := h.CourseModel.Create(...)
}
```

### 중앙 Server: 도메인 Handler를 조합

```go
// internal/service/server.go
package service

import (
    "dummy-lesson/backend/internal/service/course"
    "dummy-lesson/backend/internal/service/auth"
    "dummy-lesson/backend/internal/service/review"
)

type Server struct {
    Course     *course.Handler
    Auth       *auth.Handler
    Review     *review.Handler
}

func Handler(s *Server) http.Handler {
    mux := http.NewServeMux()

    // course 도메인 라우트
    mux.HandleFunc("GET /courses", s.Course.ListCourses)
    mux.HandleFunc("POST /courses", s.Course.CreateCourse)
    // ...

    // auth 도메인 라우트
    mux.HandleFunc("POST /login", s.Auth.Login)
    mux.HandleFunc("POST /register", s.Auth.Register)

    // review 도메인 라우트
    mux.HandleFunc("POST /courses/{CourseID}/reviews", func(w, r) {
        courseID, _ := strconv.ParseInt(r.PathValue("CourseID"), 10, 64)
        s.Review.CreateReview(w, r, courseID)
    })

    return mux
}
```

### main.go

```go
// cmd/main.go
server := &service.Server{
    Course: &course.Handler{
        CourseModel: model.NewCourseModel(conn),
        LessonModel: model.NewLessonModel(conn),
    },
    Auth: &auth.Handler{
        UserModel: model.NewUserModel(conn),
    },
    Review: &review.Handler{
        ReviewModel: model.NewReviewModel(conn),
    },
}
handler := service.Handler(server)
```

---

## glue-gen 변경

### 1. transformServiceFiles: 도메인별 처리

현재 `internal/service/` flat 변환 → 도메인별 서브디렉토리 각각 변환.

```go
func transformServiceFiles(intDir string, funcs []ServiceFunc, modulePath string) error {
    // 도메인별로 그룹화
    byDomain := groupByDomain(funcs)

    for domain, domainFuncs := range byDomain {
        if domain == "" {
            // flat: 기존 로직
            transformFlat(intDir, domainFuncs, modulePath)
        } else {
            // 도메인: 서브디렉토리 내 파일 변환
            transformDomain(intDir, domain, domainFuncs, modulePath)
        }
    }
}
```

변환 내용:
- `package service` 유지 (flat) 또는 `package {domain}` 유지 (도메인)
- flat: `func Create(...)` → `func (s *Server) Create(...)` (현행 동작 유지)
- 도메인: `func Create(...)` → `func (h *Handler) Create(...)`
- flat: `courseModel.` → `s.CourseModel.`
- 도메인: `courseModel.` → `h.CourseModel.`

현재 `transformSource` 시그니처:
```go
func transformSource(src string, models, funcs, components []string, modulePath string, xConfigs map[string]string) string
```
도메인 모드 추가 시 receiver(`s` vs `h`)와 타입(`*Server` vs `*Handler`)을 분기해야 한다.

### 2. generateServerStruct: 도메인별 Handler + 중앙 Server

현재: 단일 Server struct에 모든 모델 필드.

수정:
- 도메인이 있으면 도메인별 `handler.go` 생성 (해당 도메인이 사용하는 모델만 포함)
- 중앙 `server.go`에 도메인 Handler 필드

### 3. generateMain: 도메인별 Handler 초기화

현재: `service.Server{CourseModel: model.NewCourseModel(conn)}`.

수정: `service.Server{Course: &course.Handler{CourseModel: model.NewCourseModel(conn)}}`.

### 4. Handler(): 도메인별 라우트 등록

현재: `s.ListCourses` 직접 호출.

수정: `s.Course.ListCourses` 도메인 경유 호출.

---

## 도메인-모델 매핑

어떤 도메인이 어떤 모델을 사용하는지는 ssac ServiceFunc에서 파생한다:

```go
// ServiceFunc.Domain = "course"
// seq.Model = "Course.FindByID" → CourseModel
// seq.Model = "Lesson.ListByCourse" → LessonModel
// → course.Handler에 CourseModel, LessonModel 포함
```

`collectModels(funcs)` 를 도메인별로 수행:

```go
func collectModelsByDomain(funcs []ServiceFunc) map[string][]string {
    // domain → []modelName
}
```

---

## 하위 호환

- **flat 구조** (Domain = ""): 모든 서비스가 `service/` 에 `package service`로 생성. 현재와 동일.
- **도메인 구조** (Domain != ""): 도메인별 서브패키지. 새 코드 경로.
- **혼합**: flat + 도메인 공존 가능. flat 서비스는 Server에 직접, 도메인 서비스는 Handler 경유.

---

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `gluegen/gluegen.go` | `transformServiceFiles` 도메인별 분기 |
| `gluegen/server.go` | 도메인별 handler.go 생성, 중앙 server.go에 도메인 Handler 필드, Handler() 라우트 도메인 경유 |
| `gluegen/main_go.go` | 도메인별 Handler 초기화, 도메인 패키지 import |
| `orchestrator/gen.go` | ssac에서 받은 Domain 정보를 glue-gen에 전달 |

## 의존성

- **ssac 수정지시서007 적용** ✅ 완료 — `ServiceFunc.Domain` 필드 + 재귀 탐색 + 도메인별 package 출력
- **Phase014 완료** ✅
- **Phase015 완료** ✅

## 검증

```bash
# 1. dummy-lesson을 도메인 폴더 구조로 재배치
specs/dummy-lesson/service/
├── course/
├── lesson/
├── auth/
├── enrollment/
├── review/
└── payment/

# 2. fullend gen
fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/

# 3. 생성 구조 확인
artifacts/dummy-lesson/backend/internal/service/
├── server.go          ← package service, Server{Course, Auth, ...}
├── auth.go            ← package service, CurrentUser
├── course/
│   ├── handler.go     ← package course, Handler{CourseModel, LessonModel}
│   ├── create_course.go
│   └── list_courses.go
├── auth/
│   ├── handler.go
│   ├── login.go
│   └── register.go
└── ...

# 4. go build
cd artifacts/dummy-lesson/backend && go build ./...

# 5. flat 하위 호환 테스트 (도메인 폴더 없는 프로젝트)
fullend gen specs/flat-project/ artifacts/flat-project/
cd artifacts/flat-project/backend && go build ./...
```
