# ✅ Phase 010: 글루 코드 생성 (Server struct + main.go)

## 목표

fullend gen이 실행 가능한 서버를 산출하도록 글루 코드를 자동 생성한다.
현재 개별 생성물(oapi-codegen types/server, ssac service/model, stml pages)을 연결하는 코드가 없다.

## 현재 생성물과 갭

```
oapi-codegen   →  ServerInterface (메서드 시그니처)
ssac           →  standalone 함수 (courseModel.FindByID 등 패키지변수 참조)
ssac model     →  interface (CourseModel, UserModel 등)
stml           →  React TSX pages
```

**갭:** ServerInterface 구현체, DI, main.go, 인증 미들웨어

## 생성할 파일

```
<artifacts-dir>/
├── backend/
│   ├── api/
│   │   ├── types.gen.go        # oapi-codegen (기존)
│   │   └── server.gen.go       # oapi-codegen (기존)
│   ├── model/
│   │   └── model/models_gen.go # ssac (기존)
│   ├── service/
│   │   └── *.go                # ssac (기존)
│   ├── server.go               # ★ NEW: Server struct + 메서드 래핑
│   ├── auth.go                 # ★ NEW: 인증 미들웨어 스텁
│   └── cmd/
│       └── main.go             # ★ NEW: 엔트리포인트
└── frontend/
    ├── *.tsx                   # stml (기존)
    ├── package.json            # ★ NEW: React + Vite 의존성
    ├── vite.config.ts          # ★ NEW: Vite 설정 (API proxy 포함)
    ├── tsconfig.json           # ★ NEW: TypeScript 설정
    ├── index.html              # ★ NEW: Vite 엔트리 HTML
    └── src/
        ├── main.tsx            # ★ NEW: React 엔트리포인트
        ├── App.tsx             # ★ NEW: 라우터 + 페이지 연결
        └── api.ts              # ★ NEW: API 클라이언트 (fetch wrapper)
```

## 생성 내용

### 1. `server.go` — Server struct + 메서드 변환

ssac standalone 함수를 Server struct 메서드로 변환한다.

**입력 정보:**
- ssac model interfaces → Server struct 필드
- ssac standalone 함수 본문 → 메서드 본문 (`courseModel` → `s.courseModel`)
- oapi-codegen ServerInterface → 시그니처 일치 확인
- ssac `@component`, `@func` → 추가 필드

```go
package backend

type Server struct {
    courseModel      model.CourseModel
    userModel        model.UserModel
    lessonModel      model.LessonModel
    enrollmentModel  model.EnrollmentModel
    reviewModel      model.ReviewModel
    paymentModel     model.PaymentModel
    notification     model.Notification        // @component
    issueToken       func(int64) (model.Token, error)  // @func
    hashPassword     func(string) (string, error)      // @func
}

func (s *Server) GetCourse(w http.ResponseWriter, r *http.Request, courseID int64) {
    // ssac 함수 본문에서 courseModel → s.courseModel 치환
    course, err := s.courseModel.FindByID(courseID)
    ...
}

// 컴파일 타임 체크
var _ api.ServerInterface = (*Server)(nil)
```

**변환 규칙:**
- `{model}Model` → `s.{model}Model` (모든 모델 참조)
- `currentUser` → `s.currentUser(r)` (인증 컨텍스트에서 추출)
- `notification` → `s.notification` (component 참조)
- `issueToken(...)` → `s.issueToken(...)` (func 참조)
- `authz.Check(...)` → `s.authz.Check(...)` (authorize 시퀀스)
- path parameter는 ssac이 이미 함수 인자로 생성 (수정지시서002 완료)

### 2. `auth.go` — 인증 미들웨어 스텁

```go
package backend

type CurrentUser struct {
    UserID int64
    // OpenAPI securitySchemes에서 파생
}

func (s *Server) currentUser(r *http.Request) *CurrentUser {
    // TODO: JWT 토큰 파싱 구현
    return nil
}
```

### 3. `cmd/main.go` — 서버 엔트리포인트

기본 DB는 PostgreSQL. CLI 옵션으로 변경 가능.

```go
package main

import (
    "database/sql"
    "flag"
    "log"
    "net/http"

    _ "github.com/lib/pq"           // postgres (기본)
    // _ "github.com/go-sql-driver/mysql"  // mysql 옵션
)

func main() {
    addr := flag.String("addr", ":8080", "listen address")
    dsn := flag.String("dsn", "postgres://localhost:5432/app?sslmode=disable", "database connection string")
    dbDriver := flag.String("db", "postgres", "database driver (postgres, mysql)")
    flag.Parse()

    db, err := sql.Open(*dbDriver, *dsn)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    server := NewServer(db)
    handler := api.Handler(server)
    log.Printf("listening on %s", *addr)
    log.Fatal(http.ListenAndServe(*addr, handler))
}
```

### 4. 프론트엔드 셋업 — React + Vite

stml이 생성한 TSX 파일이 바로 동작하도록 프로젝트 셋업을 생성한다.

**package.json:**
```json
{
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  },
  "dependencies": {
    "react": "^19",
    "react-dom": "^19",
    "react-router-dom": "^7"
  },
  "devDependencies": {
    "@types/react": "^19",
    "@types/react-dom": "^19",
    "@vitejs/plugin-react": "^4",
    "typescript": "^5",
    "vite": "^6"
  }
}
```

**vite.config.ts:**
```ts
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080'  // Go 백엔드로 프록시
    }
  }
})
```

**App.tsx** — stml 페이지를 라우터에 연결:
```tsx
// OpenAPI paths에서 파생
<Route path="/courses" element={<CourseListPage />} />
<Route path="/courses/:courseID" element={<CourseDetailPage />} />
<Route path="/login" element={<LoginPage />} />
```

**api.ts** — OpenAPI 엔드포인트별 fetch wrapper:
```ts
export async function listCourses(params?) { return fetch('/api/courses?...') }
export async function getCourse(courseID: number) { return fetch(`/api/courses/${courseID}`) }
```

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `artifacts/internal/orchestrator/gen.go` | `genGlue()` 함수 추가, Gen()에서 호출 |
| `artifacts/internal/gluegen/gluegen.go` | ★ NEW: 글루 코드 생성 엔트리 |
| `artifacts/internal/gluegen/server.go` | ★ NEW: Server struct + 메서드 변환 |
| `artifacts/internal/gluegen/main_go.go` | ★ NEW: main.go 생성 |
| `artifacts/internal/gluegen/auth.go` | ★ NEW: auth 스텁 생성 |
| `artifacts/internal/gluegen/frontend.go` | ★ NEW: React + Vite 셋업 생성 |

## 구현 전략

### 메서드 변환 방식

ssac `generator.Generate()`가 반환하는 각 파일의 소스 코드를 Go AST로 파싱하여:
1. 함수 선언에서 receiver `(s *Server)` 추가
2. 함수 본문의 식별자 치환 (`courseModel` → `s.courseModel`)
3. import 조정

또는 더 간단하게 **문자열 치환**:
1. `func FuncName(` → `func (s *Server) FuncName(`
2. 모든 `{model}Model.` → `s.{model}Model.`
3. `currentUser` → `s.currentUser(r)`

문자열 치환이 더 단순하고 ssac 출력 형태가 예측 가능하므로 이 방식을 채택한다.

### 필요 정보 소스

| 정보 | 소스 |
|---|---|
| 모델 인터페이스 목록 | ssac `GenerateModelInterfaces` 결과 또는 SymbolTable.Models |
| 함수 시그니처 | ssac `ServiceFunc` + SymbolTable.Operations (path params) |
| 함수 본문 | ssac `generator.Generate()` 출력 파일 |
| component/func 목록 | SymbolTable.Funcs, model 파일의 interface 정의 |
| ServerInterface | oapi-codegen 생성 결과 (참조용, 시그니처는 ssac이 이미 맞춤) |

## gen 파이프라인 순서 (최종)

```
1. sqlc         ← DB 모델 + 쿼리 구현 (import)
2. oapi-gen     ← OpenAPI 타입 + 서버 스텁 (import)
3. ssac-gen     ← 서비스 함수 (standalone)
4. ssac-model   ← 모델 인터페이스
5. stml-gen     ← React TSX 페이지
6. glue-gen     ← Server struct + main.go + React/Vite 셋업 ★ NEW
7. terraform    ← HCL 포맷팅 (외부 도구, 선택)
```

glue-gen은 ssac-gen 출력을 입력으로 사용하므로 반드시 ssac-gen 이후에 실행한다.

## 의존성

- ssac, stml 변경 없음
- ssac 수정지시서002 (path parameter 시그니처) 완료 전제

## 검증 방법

1. `go build ./artifacts/cmd/... ./artifacts/internal/...` 성공
2. `fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/` 실행 시 `✓ glue-gen` 출력
3. 생성된 `server.go`에 `var _ api.ServerInterface = (*Server)(nil)` 포함
4. 생성된 `cmd/main.go`에 `api.Handler(server)` + `--db` 플래그 포함
5. 생성된 `frontend/package.json`에 react, vite 의존성 포함
6. 생성된 `frontend/App.tsx`에 OpenAPI paths 기반 라우트 포함
7. (이상적) `go build ./artifacts/dummy-lesson/backend/...` 컴파일 성공
