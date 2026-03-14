# Phase028: genapi 인터페이스 분리 + gen 패키지 재구조화 ✅ 완료

## 목표

코드 생성의 백엔드 의존성을 분리하고, OpenAPI 계약 기반 생성기(React, Hurl)를 독립 패키지로 추출한다.
**동작 변경 없음** — 생성 결과는 기존과 동일해야 한다.

## 동기

현재 `gluegen/` 패키지의 문제:

1. **Go+Gin 하드코딩** — 모든 gen 함수가 Go import 경로, Gin 핸들러 시그니처를 직접 생성. 다른 언어(Java, PHP 등) 추가 시 `gluegen/` 전체를 복제해야 함
2. **관심사 혼재** — 백엔드 코드 생성(Go+Gin)과 OpenAPI 계약 기반 생성(React 프론트엔드, Hurl 테스트)이 한 패키지에 섞여 있음. React/Hurl은 백엔드가 무엇이든 OpenAPI 계약만 맞으면 동작하므로 독립적이어야 함
3. **중복 struct** — `GlueInput`, `CrossValidateInput`, `orchestrator.ParsedSSOTs` 3개 struct가 동일 파싱 결과를 중복 보유

## 의존성

Phase026 (crosscheck rulebook) 완료 후 실행.
crosscheck도 `genapi.ParsedSSOTs`를 입력으로 전환하므로 Phase026의 `CrossValidateInput`이 정리된 상태여야 한다.

---

## 설계

### 1. `internal/genapi/` 패키지 (신규)

```go
package genapi

import (
    "github.com/getkin/kin-openapi/openapi3"

    "github.com/geul-org/fullend/internal/funcspec"
    "github.com/geul-org/fullend/internal/policy"
    "github.com/geul-org/fullend/internal/projectconfig"
    "github.com/geul-org/fullend/internal/statemachine"
    ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
    ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
    stmlparser "github.com/geul-org/fullend/internal/stml/parser"
)

// ParsedSSOTs holds all SSOT parsing results.
// orchestrator.ParseAll() populates this; crosscheck and gen consume it.
type ParsedSSOTs struct {
    Config           *projectconfig.ProjectConfig
    OpenAPIDoc       *openapi3.T
    SymbolTable      *ssacvalidator.SymbolTable
    ServiceFuncs     []ssacparser.ServiceFunc
    STMLPages        []stmlparser.PageSpec
    StateDiagrams    []*statemachine.StateDiagram  // crosscheck 기존 필드명 유지
    Policies         []*policy.Policy
    ProjectFuncSpecs []funcspec.FuncSpec            // crosscheck 기존 필드명 유지
    FullendPkgSpecs  []funcspec.FuncSpec            // crosscheck 기존 필드명 유지
    HurlFiles        []string
    ModelDir         string
    StatesErr        error
}

// GenConfig holds generation settings (not parsing results).
type GenConfig struct {
    ArtifactsDir string
    SpecsDir     string
    ModulePath   string
}

// STMLGenOutput holds STML generator output (not parse results).
// Populated by orchestrator after stml.Generate(), consumed by react gen.
type STMLGenOutput struct {
    Deps     map[string]string // npm dependencies
    Pages    []string          // page names
    PageOps  map[string]string // page file → primary operationID
}

// Backend generates backend code from parsed SSOTs.
type Backend interface {
    Generate(parsed *ParsedSSOTs, cfg *GenConfig) error
}
```

필드명 통일 원칙:
- `StateDiagrams` (orchestrator의 `States` → 이름 변경) — crosscheck Check 함수 16개가 `input.StateDiagrams`를 참조하므로
- `ProjectFuncSpecs`, `FullendPkgSpecs` — crosscheck에서 이 이름으로 참조
- orchestrator `ParseAll()` 내부에서 `p.States = diagrams` → `p.StateDiagrams = diagrams`로 변경

### 2. 의존성 방향

```
genapi ← orchestrator   (ParseAll → *genapi.ParsedSSOTs 반환)
genapi ← crosscheck     (Run 입력으로 사용)
genapi ← gen            (오케스트레이터)
genapi ← gen/gogin      (Backend 구현)
genapi ← gen/react      (ParsedSSOTs + STMLGenOutput 소비)
genapi ← gen/hurl       (ParsedSSOTs 소비)
```

모든 화살표가 `genapi`를 향함 — 순환 불가.

### 3. 패키지 구조

```
internal/
├── genapi/           # ParsedSSOTs, GenConfig, STMLGenOutput, Backend interface
├── gen/
│   ├── gen.go        # 오케스트레이터: selectBackend + react + hurl 호출
│   ├── gogin/        # Go+Gin 백엔드 (Backend 구현)
│   │   ├── gogin.go  # GoGin struct, Generate, transformSource, collectModels
│   │   ├── server.go
│   │   ├── domain.go
│   │   ├── main_go.go
│   │   ├── auth.go
│   │   ├── middleware.go
│   │   ├── model_impl.go
│   │   ├── attach.go
│   │   ├── state.go
│   │   ├── authz.go
│   │   ├── queryopts.go
│   │   └── queryopts_test.go
│   ├── react/        # OpenAPI → React 프론트엔드 (백엔드 무관)
│   │   └── react.go
│   └── hurl/         # OpenAPI → Hurl smoke test (백엔드 무관)
│       ├── hurl.go
│       └── hurl_util.go
```

### 4. 파일 이동 맵

| 원본 (gluegen/) | 이동 | 분류 |
|---|---|---|
| `gluegen.go` | `gen/gogin/gogin.go` | Go+Gin 유틸 (transformSource, collectModels 등) |
| `server.go` | `gen/gogin/server.go` | Go+Gin Server struct 생성 |
| `domain.go` | `gen/gogin/domain.go` | Go+Gin Domain Handler 생성 |
| `main_go.go` | `gen/gogin/main_go.go` | Go+Gin main.go 생성 |
| `auth.go` | `gen/gogin/auth.go` | Go+Gin auth stub 생성 |
| `middlewaregen.go` | `gen/gogin/middleware.go` | Go+Gin bearerAuth 미들웨어 생성 |
| `model_impl.go` | `gen/gogin/model_impl.go` | Go+Gin Model 구현체 생성 |
| `attach.go` | `gen/gogin/attach.go` | Go+Gin SSaC directive 삽입 |
| `stategen.go` | `gen/gogin/state.go` | Go+Gin 상태 머신 코드 생성 |
| `authzgen.go` | `gen/gogin/authz.go` | Go+Gin authz 패키지 생성 |
| `queryopts.go` | `gen/gogin/queryopts.go` | Go+Gin QueryOpts 생성 |
| `queryopts_test.go` | `gen/gogin/queryopts_test.go` | QueryOpts 테스트 |
| `frontend.go` | `gen/react/react.go` | OpenAPI → React (백엔드 무관) |
| `hurl.go` | `gen/hurl/hurl.go` | OpenAPI → Hurl (백엔드 무관) |
| `hurl_util.go` | `gen/hurl/hurl_util.go` | Hurl 유틸 (백엔드 무관) |

`gluegen/` 디렉토리는 완전히 제거된다.

### 5. `internal/gen/gen.go` (신규)

```go
package gen

import (
    "github.com/geul-org/fullend/internal/genapi"
    "github.com/geul-org/fullend/internal/gen/gogin"
    "github.com/geul-org/fullend/internal/gen/react"
    "github.com/geul-org/fullend/internal/gen/hurl"
    "github.com/geul-org/fullend/internal/projectconfig"
)

// Generate creates all artifacts from parsed SSOTs.
func Generate(parsed *genapi.ParsedSSOTs, cfg *genapi.GenConfig, stmlOut *genapi.STMLGenOutput) error {
    // 1. 백엔드 코드 생성
    backend := selectBackend(parsed.Config)
    if err := backend.Generate(parsed, cfg); err != nil {
        return err
    }
    // 2. React 프론트엔드 생성 (OpenAPI 계약 기반, 백엔드 무관)
    if err := react.Generate(parsed, cfg, stmlOut); err != nil {
        return err
    }
    // 3. Hurl smoke test 생성 (OpenAPI 계약 기반, 백엔드 무관)
    if err := hurl.Generate(parsed, cfg); err != nil {
        return err
    }
    return nil
}

func selectBackend(cfg *projectconfig.ProjectConfig) genapi.Backend {
    // 추후 cfg.Backend 필드로 분기
    return &gogin.GoGin{}
}
```

### 6. `internal/gen/gogin/gogin.go`

```go
package gogin

import "github.com/geul-org/fullend/internal/genapi"

// GoGin implements genapi.Backend for Go + Gin framework.
type GoGin struct{}

func (g *GoGin) Generate(parsed *genapi.ParsedSSOTs, cfg *genapi.GenConfig) error {
    // 현재 gluegen.Generate()의 Go+Gin 부분을 그대로 이동
    // domain/flat 분기, transformSource, collectModels 등 포함
    return nil
}

// GenerateStateMachines generates Go state machine packages.
// orchestrator에서 직접 호출 (Backend.Generate와 별도 단계).
func GenerateStateMachines(diagrams []*statemachine.StateDiagram, artifactsDir, modulePath string) error {
    // gluegen.GenerateStateMachines 이동
}

// GenerateAuthzPackage copies .rego files for runtime loading.
// orchestrator에서 직접 호출 (Backend.Generate와 별도 단계).
func GenerateAuthzPackage(policies []*policy.Policy, artifactsDir string) error {
    // gluegen.GenerateAuthzPackage 이동
}
```

### 7. `internal/orchestrator/parsed.go` 변경

기존 `ParsedSSOTs` struct를 `genapi.ParsedSSOTs`로 교체:

```go
package orchestrator

import "github.com/geul-org/fullend/internal/genapi"

// ParseAll returns *genapi.ParsedSSOTs (not orchestrator-local struct).
func ParseAll(root string, detected []DetectedSSOT, skip map[SSOTKind]bool) *genapi.ParsedSSOTs {
    p := &genapi.ParsedSSOTs{}
    // ... 기존 파싱 로직 동일
    // 필드명 변경: p.States → p.StateDiagrams
    // 필드명 변경: p.FuncSpecs → p.ProjectFuncSpecs
    // 필드명 변경: p.PkgFuncSpecs → p.FullendPkgSpecs
    return p
}
```

orchestrator 내부에서 `parsed.States` → `parsed.StateDiagrams` 등 필드 참조 변경 필요:
- `validate.go`: `parsed.States` → `parsed.StateDiagrams`
- `gen.go`: `parsed.States` → `parsed.StateDiagrams`
- `status.go`: `parsed.States` → `parsed.StateDiagrams`
- `chain.go`: `parsed.States` → `parsed.StateDiagrams`

### 8. `internal/crosscheck/types.go` 변경

`CrossValidateInput`의 파싱 관련 필드를 `*genapi.ParsedSSOTs` 임베딩으로 교체:

```go
type CrossValidateInput struct {
    *genapi.ParsedSSOTs
    // crosscheck 전용 설정 (파싱 결과 아님)
    DTOTypes        map[string]bool
    Middleware      []string
    Archived        *ArchivedInfo
    Claims          map[string]string
    QueueBackend    string
    AuthzPackage    string
    SensitiveCols   map[string]map[string]bool
    NoSensitiveCols map[string]map[string]bool
    Roles           []string
}
```

genapi.ParsedSSOTs의 필드명이 기존 CrossValidateInput과 동일하므로 (`OpenAPIDoc`, `SymbolTable`, `ServiceFuncs`, `StateDiagrams`, `Policies`, `HurlFiles`, `ProjectFuncSpecs`, `FullendPkgSpecs`), 임베딩으로 기존 Check 함수 16개는 **변경 없이 동작**.

### 9. `internal/orchestrator/gen.go` 변경

```go
// 변경 전
gluegen.Generate(input)
gluegen.GenerateStateMachines(diagrams, artifactsDir, modulePath)
gluegen.GenerateAuthzPackage(policies, artifactsDir)

// 변경 후
gen.Generate(parsed, cfg, stmlOut)
gogin.GenerateStateMachines(parsed.StateDiagrams, artifactsDir, modulePath)
gogin.GenerateAuthzPackage(parsed.Policies, artifactsDir)
```

GlueInput 구성 코드 제거. `parsed` (*genapi.ParsedSSOTs)를 직접 전달.
STML gen 출력(`STMLDeps`, `STMLPages`, `STMLPageOps`)은 `genapi.STMLGenOutput`으로 묶어 전달.

---

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/genapi/genapi.go` | 신규 — ParsedSSOTs, GenConfig, STMLGenOutput, Backend interface |
| `internal/gen/gen.go` | 신규 — 오케스트레이터 (selectBackend + react + hurl 호출) |
| `internal/gen/gogin/gogin.go` | 신규 — GoGin struct, Generate, GenerateStateMachines, GenerateAuthzPackage |
| `internal/gen/gogin/*.go` | gluegen/에서 Go+Gin 전용 11개 파일 이동 |
| `internal/gen/gogin/queryopts_test.go` | gluegen/queryopts_test.go 이동 |
| `internal/gen/react/react.go` | gluegen/frontend.go 이동. 시그니처: (ParsedSSOTs, GenConfig, STMLGenOutput) |
| `internal/gen/hurl/hurl.go` | gluegen/hurl.go 이동. 시그니처: (ParsedSSOTs, GenConfig) |
| `internal/gen/hurl/hurl_util.go` | gluegen/hurl_util.go 이동 |
| `internal/gluegen/` | 디렉토리 삭제 |
| `internal/orchestrator/parsed.go` | ParsedSSOTs struct 삭제, genapi.ParsedSSOTs 사용. 필드명 변경 (States→StateDiagrams 등) |
| `internal/orchestrator/validate.go` | parsed.States → parsed.StateDiagrams 등 필드 참조 변경 |
| `internal/orchestrator/gen.go` | gluegen → gen/gogin 호출 변경. GlueInput 제거. STMLGenOutput 전달 |
| `internal/orchestrator/status.go` | parsed.States → parsed.StateDiagrams 필드 참조 변경 |
| `internal/orchestrator/chain.go` | parsed.States → parsed.StateDiagrams 필드 참조 변경 |
| `internal/crosscheck/types.go` | CrossValidateInput에 genapi.ParsedSSOTs 임베딩. 중복 필드 제거, import 변경 |

## 변경하지 않는 파일

- crosscheck Check 함수 16개 + rules.go — `input.OpenAPIDoc`, `input.StateDiagrams` 등 접근 방식이 임베딩으로 유지
- gogin/ 이동 파일의 내부 로직 — 시그니처만 GlueInput → ParsedSSOTs+GenConfig 변경
- hurl 이동 파일의 내부 로직 — 시그니처만 GlueInput → ParsedSSOTs+GenConfig 변경

## 검증

1. `go build ./...` — 컴파일 통과
2. `go test ./...` — 기존 테스트 전량 통과
3. `go run ./cmd/fullend validate specs/gigbridge` — 기존 동일 출력
4. `go run ./cmd/fullend gen specs/gigbridge artifacts/gigbridge` — 기존 동일 산출물
5. `go vet ./...` — 정적 분석 통과

## 향후 확장

다른 백엔드 추가 시:
1. `internal/gen/javaspring/` 패키지 생성
2. `genapi.Backend` 구현
3. `fullend.yaml`에 `backend: java-spring` 설정 추가
4. `gen/gen.go`의 `selectBackend`에서 config 기반 분기

다른 프론트엔드 추가 시:
1. `internal/gen/vue/` 등 패키지 생성 (OpenAPI 계약 기반)
2. `gen/gen.go`에서 config 기반 프론트엔드 선택 분기 추가
