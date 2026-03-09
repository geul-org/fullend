# ✅ Phase 011: sqlc.yaml 자동 생성 + glue-gen 강화 + 폴더 구조 개선

## 목표

ssac 수정지시서003, stml 수정지시서001 완료에 따라 fullend 측 코드를 업데이트한다.

1. DDL 감지 시 `sqlc.yaml`을 자동 생성하여 sqlc generate 실행
2. stml `Generate()` 새 시그니처 적용 (옵션 + 의존성 반환)
3. glue-gen 강화: api.ts 객체 export, package.json 의존성 병합, main.go 완성
4. 출력 폴더 구조를 `internal/` 기반으로 정리

## 전제 (완료된 수정지시서)

- **ssac 003**: Model 인터페이스 DDL 타입 반영, JSON body 파싱, `@id new` → `nil`
- **stml 001**: `GenerateOptions.APIImportPath`, `GenerateResult.Dependencies`, `GenerateOptions.UseClient`

---

## 1. sqlc.yaml 자동 생성

### 현재

DDL 파일이 있어도 `sqlc.yaml`이 없으면 `sqlc generate`를 스킵한다.

### 변경

DDL 감지 시 fullend가 `sqlc.yaml`을 자동 생성한 뒤 `sqlc generate`를 실행한다.

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "db/"
    queries: "db/queries/"
    gen:
      go:
        package: "db"
        out: "<artifacts-dir>/backend/internal/db"
        sql_package: "database/sql"
        emit_json_tags: true
        emit_empty_slices: true
```

**엔진 판별**: DDL 파일 내용 기반으로 PostgreSQL/MySQL 자동 감지.
- `SERIAL`, `BIGSERIAL`, `TIMESTAMPTZ`, `TEXT` → PostgreSQL
- `AUTO_INCREMENT`, `DATETIME`, `ENGINE=InnoDB` → MySQL
- 판별 불가 시 → PostgreSQL (기본값)

**쿼리 디렉토리**: `db/queries/` 존재 여부 확인. 없으면 sqlc.yaml 생성을 스킵하고 경고.

### 구현

| 파일 | 변경 |
|---|---|
| `orchestrator/gen.go` `genSqlc()` | sqlc.yaml 없으면 자동 생성 후 실행 |
| `orchestrator/sqlc_config.go` | ★ NEW: sqlc.yaml 생성 로직 + DB 엔진 감지 |

---

## 2. stml Generate() 새 시그니처 적용

### 현재

```go
stmlgenerator.Generate(pages, specsDir, outDir)
```

### 변경

```go
result, err := stmlgenerator.Generate(pages, specsDir, outDir, stmlgenerator.GenerateOptions{
    APIImportPath: "../api",
    UseClient:     false,
})
// result.Dependencies → package.json에 병합
```

### 구현

| 파일 | 변경 |
|---|---|
| `orchestrator/gen.go` `genSTML()` | 새 시그니처 + 옵션 전달 |
| `orchestrator/gen.go` `Gen()` | stml 의존성 결과를 genGlue에 전달 |

---

## 3. 출력 폴더 구조 개선

### 현재

```
artifacts/dummy-lesson/
├── backend/
│   ├── api/                    # oapi-codegen
│   ├── service/                # ssac gen (flat)
│   ├── model/model/            # ssac model (이중 경로)
│   ├── server.go               # glue
│   ├── auth.go                 # glue
│   └── cmd/main.go             # glue
└── frontend/
    ├── *.tsx                   # stml gen (root에 산재)
    ├── src/{App,api,main}.tsx  # glue
    └── package.json            # glue
```

### 변경 후

```
artifacts/dummy-lesson/
├── backend/
│   ├── cmd/main.go             # glue
│   └── internal/
│       ├── api/                # oapi-codegen (types.gen.go, server.gen.go)
│       ├── db/                 # sqlc generate (models.go, querier.go, *.sql.go)
│       ├── service/            # ssac gen → glue 변환 (Server 메서드)
│       ├── model/              # ssac model (인터페이스)
│       ├── server.go           # glue (Server struct)
│       └── auth.go             # glue (CurrentUser + stub)
└── frontend/
    ├── index.html              # glue
    ├── package.json            # glue (stml 의존성 병합)
    ├── tsconfig.json           # glue
    ├── vite.config.ts          # glue
    └── src/
        ├── main.tsx            # glue
        ├── App.tsx             # glue
        ├── api.ts              # glue
        └── pages/              # stml gen
            ├── login-page.tsx
            ├── course-list-page.tsx
            └── ...
```

### 변경사항

| 항목 | 현재 경로 | 변경 경로 | 방법 |
|---|---|---|---|
| oapi-codegen | `backend/api/` | `backend/internal/api/` | genOpenAPI outDir 변경 |
| sqlc | (없음) | `backend/internal/db/` | sqlc.yaml out 경로 지정 |
| ssac service | `backend/service/` | `backend/internal/service/` | genSSaC outDir 변경 |
| ssac model | `backend/model/model/` | `backend/internal/model/` | genSSaC modelOutDir 변경 |
| server.go | `backend/` | `backend/internal/` | glue 출력 경로 변경 |
| auth.go | `backend/` | `backend/internal/` | glue 출력 경로 변경 |
| stml pages | `frontend/` (root) | `frontend/src/pages/` | genSTML outDir 변경 |

### 구현

| 파일 | 변경 |
|---|---|
| `orchestrator/gen.go` | 모든 outDir을 `internal/` 기반으로 수정 |
| `gluegen/gluegen.go` | artifactsDir 기준 경로 수정 |
| `gluegen/server.go` | package명 변경: `backend` → 실제 패키지명 결정 |
| `gluegen/auth.go` | 동일 |
| `gluegen/main_go.go` | import 경로 수정 |
| `gluegen/frontend.go` | App.tsx import 경로를 `./pages/` 기반으로 |

---

## 4. glue-gen 강화

### 4-1. api.ts 객체 export

stml이 `api.ListCourses()` (PascalCase)로 호출하므로, api.ts에 namespace 객체를 추가한다.

```typescript
// 개별 함수 (camelCase)
async function listCourses(...) { ... }
async function getCourse(...) { ... }

// stml 호환 객체 (PascalCase)
export const api = {
  ListCourses: listCourses,
  GetCourse: getCourse,
  CreateCourse: createCourse,
  // ...
}
```

### 4-2. package.json 의존성 병합

stml `GenerateResult.Dependencies`를 package.json에 병합한다.

```go
func writePackageJSON(dir string, stmlDeps map[string]string) error {
    deps := map[string]string{
        "react":            "^18",
        "react-dom":        "^18",
        "react-router-dom": "^6",
    }
    // stml 의존성 병합
    for k, v := range stmlDeps {
        deps[k] = v
    }
    // ...
}
```

기대 결과:
```json
{
  "dependencies": {
    "react": "^18",
    "react-dom": "^18",
    "react-router-dom": "^6",
    "@tanstack/react-query": "^5",
    "react-hook-form": "^7"
  }
}
```

### 4-3. main.go 완성

현재 TODO 주석으로 남겨둔 Server 초기화 + Handler 등록을 생성한다.

```go
// TODO 주석 제거, 실제 코드 생성
server := &internal.Server{
    // model 필드 초기화는 sqlc/DB 타입 통합 후
}
handler := api.Handler(server)
log.Printf("listening on %s", *addr)
log.Fatal(http.ListenAndServe(*addr, handler))
```

### 4-4. App.tsx import 경로 수정

stml 페이지가 `src/pages/`로 이동하므로 import 경로 변경:

```typescript
// 변경 전
import ListCoursesPage from './list-courses-page'

// 변경 후
import ListCoursesPage from './pages/list-courses-page'
```

### 구현

| 파일 | 변경 |
|---|---|
| `gluegen/frontend.go` `writeAPIClient()` | api 객체 export 추가 |
| `gluegen/frontend.go` `writePackageJSON()` | stmlDeps 파라미터 추가 |
| `gluegen/frontend.go` `writeAppTSX()` | import 경로에 `pages/` prefix |
| `gluegen/main_go.go` | Server 초기화 + Handler 등록 코드 생성 |
| `gluegen/gluegen.go` `GlueInput` | stmlDeps 필드 추가 |

---

## 변경 파일 목록

| 파일 | 변경 유형 |
|---|---|
| `orchestrator/gen.go` | 수정: 폴더 경로 + stml 시그니처 + stmlDeps 전달 |
| `orchestrator/sqlc_config.go` | ★ NEW: sqlc.yaml 자동 생성 |
| `gluegen/gluegen.go` | 수정: GlueInput에 stmlDeps + 경로 수정 |
| `gluegen/frontend.go` | 수정: api 객체 export + deps 병합 + import 경로 |
| `gluegen/main_go.go` | 수정: Server 초기화 코드 완성 |
| `gluegen/server.go` | 수정: package명/경로 조정 |
| `gluegen/auth.go` | 수정: package명/경로 조정 |

## 의존성

- ssac 수정지시서003 완료 (Model 타입, JSON body, @id new)
- stml 수정지시서001 완료 (GenerateOptions, GenerateResult)
- go.mod `replace` 디렉티브 업데이트 (ssac, stml 최신 반영)

## 검증 방법

1. `go build ./artifacts/cmd/... ./artifacts/internal/...` 성공
2. `fullend gen specs/dummy-lesson/ artifacts/dummy-lesson/` 실행:
   - `✓ sqlc` — sqlc.yaml 자동 생성 + DB 모델 생성 (sqlc 설치 시)
   - `✓ oapi-gen` — `backend/internal/api/` 에 생성
   - `✓ ssac-gen` — `backend/internal/service/` 에 생성
   - `✓ ssac-model` — `backend/internal/model/` 에 생성
   - `✓ stml-gen` — `frontend/src/pages/` 에 생성
   - `✓ glue-gen` — server.go, auth.go, main.go, frontend setup
3. 생성된 `package.json`에 `@tanstack/react-query`, `react-hook-form` 포함
4. 생성된 `api.ts`에 `export const api = { ... }` 포함
5. 생성된 `App.tsx`의 import가 `./pages/` 경로 사용
6. 생성된 `main.go`에 Server 초기화 + Handler 등록 코드 포함
7. `backend/internal/` 구조 확인: api/, db/, service/, model/, server.go, auth.go
