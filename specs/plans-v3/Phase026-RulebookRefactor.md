# Phase026: Crosscheck Rulebook 패턴 리팩토링

## 목표

crosscheck의 15개 Check 함수를 Rulebook 패턴으로 전환한다.
**동작 변경 없음** — 구조만 바꾸고, 기존 테스트 전량 통과해야 한다.

## 동기

현재 `Run()` 함수는 15개 if-block의 나열이다:
- 규칙 추가마다 Run()에 if-guard + 함수 호출 수동 추가 필요
- 개별 rule 스킵 불가 (SSOT kind 단위만 가능)
- rule 메타데이터(Source/Target SSOT) 없음 → reporter가 그룹핑 불가

## 설계

### Rule struct

```go
// types.go에 추가
type Rule struct {
    Name     string // "OpenAPI ↔ DDL", "SSaC → OpenAPI", ...
    Source   string // "OpenAPI", "SSaC", "Policy", "States", "Config", "Scenario", "DDL"
    Target   string // "DDL", "OpenAPI", ... ("" = standalone)
    Requires func(*CrossValidateInput) bool
    Check    func(*CrossValidateInput) []CrossError
}
```

### rules.go (신규)

15개 Rule 등록. 기존 Check 함수를 wrapper로 감싼다.

```go
var rules = []Rule{
    {
        Name: "OpenAPI ↔ DDL", Source: "OpenAPI", Target: "DDL",
        Requires: func(in *CrossValidateInput) bool {
            return in.OpenAPIDoc != nil && in.SymbolTable != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckOpenAPIDDL(in.OpenAPIDoc, in.SymbolTable,
                in.ServiceFuncs, in.SensitiveCols)
        },
    },
    {
        Name: "SSaC ↔ DDL", Source: "SSaC", Target: "DDL",
        Requires: func(in *CrossValidateInput) bool {
            return in.ServiceFuncs != nil && in.SymbolTable != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckSSaCDDL(in.ServiceFuncs, in.SymbolTable, in.DTOTypes)
        },
    },
    {
        Name: "SSaC ↔ OpenAPI", Source: "SSaC", Target: "OpenAPI",
        Requires: func(in *CrossValidateInput) bool {
            return in.ServiceFuncs != nil && in.SymbolTable != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            all := append(in.FullendPkgSpecs, in.ProjectFuncSpecs...)
            return CheckSSaCOpenAPI(in.ServiceFuncs, in.SymbolTable,
                in.OpenAPIDoc, all)
        },
    },
    {
        Name: "States ↔ SSaC/DDL/OpenAPI", Source: "States", Target: "SSaC",
        Requires: func(in *CrossValidateInput) bool {
            return len(in.StateDiagrams) > 0
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckStates(in.StateDiagrams, in.ServiceFuncs,
                in.SymbolTable, in.OpenAPIDoc)
        },
    },
    {
        Name: "Policy ↔ SSaC/DDL/States", Source: "Policy", Target: "SSaC",
        Requires: func(in *CrossValidateInput) bool {
            return len(in.Policies) > 0
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckPolicy(in.Policies, in.ServiceFuncs,
                in.SymbolTable, in.StateDiagrams)
        },
    },
    {
        Name: "Scenario → OpenAPI", Source: "Scenario", Target: "OpenAPI",
        Requires: func(in *CrossValidateInput) bool {
            return len(in.HurlFiles) > 0 && in.OpenAPIDoc != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckHurlFiles(in.HurlFiles, in.OpenAPIDoc)
        },
    },
    {
        Name: "SSaC → Func", Source: "SSaC", Target: "Func",
        Requires: func(in *CrossValidateInput) bool {
            return in.ServiceFuncs != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckFuncs(in.ServiceFuncs, in.FullendPkgSpecs,
                in.ProjectFuncSpecs, in.SymbolTable, in.OpenAPIDoc)
        },
    },
    {
        Name: "Config → OpenAPI", Source: "Config", Target: "OpenAPI",
        Requires: func(in *CrossValidateInput) bool {
            return in.OpenAPIDoc != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckMiddleware(in.Middleware, in.OpenAPIDoc)
        },
    },
    {
        Name: "SSaC → Config", Source: "SSaC", Target: "Config",
        Requires: func(in *CrossValidateInput) bool {
            return in.ServiceFuncs != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckClaims(in.ServiceFuncs, in.Claims)
        },
    },
    {
        Name: "Policy → Config (claims)", Source: "Policy", Target: "Config",
        Requires: func(in *CrossValidateInput) bool {
            return len(in.Policies) > 0 && in.Claims != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckClaimsRego(in.Policies, in.Claims)
        },
    },
    {
        Name: "DDL → SSaC (coverage)", Source: "DDL", Target: "SSaC",
        Requires: func(in *CrossValidateInput) bool {
            return in.SymbolTable != nil && in.ServiceFuncs != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckDDLCoverage(in.SymbolTable, in.ServiceFuncs, in.Archived)
        },
    },
    {
        Name: "SSaC Queue", Source: "SSaC", Target: "",
        Requires: func(in *CrossValidateInput) bool {
            return in.ServiceFuncs != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckQueue(in.ServiceFuncs, in.QueueBackend)
        },
    },
    {
        Name: "SSaC → Authz", Source: "SSaC", Target: "Func",
        Requires: func(in *CrossValidateInput) bool {
            return in.ServiceFuncs != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckAuthz(in.ServiceFuncs, in.AuthzPackage)
        },
    },
    {
        Name: "DDL Sensitive", Source: "DDL", Target: "",
        Requires: func(in *CrossValidateInput) bool {
            return in.SymbolTable != nil
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckSensitiveColumns(in.SymbolTable,
                in.SensitiveCols, in.NoSensitiveCols)
        },
    },
    {
        Name: "Policy → Config (roles)", Source: "Policy", Target: "Config",
        Requires: func(in *CrossValidateInput) bool {
            return len(in.Policies) > 0 && len(in.Roles) > 0
        },
        Check: func(in *CrossValidateInput) []CrossError {
            return CheckRoles(in.Policies, in.Roles)
        },
    },
}
```

### crosscheck.go 변경

```go
// Run executes all registered cross-validation rules.
func Run(input *CrossValidateInput) []CrossError {
    return RunRules(input, nil)
}

// RunRules executes rules, skipping names in skipRules.
func RunRules(input *CrossValidateInput, skipRules map[string]bool) []CrossError {
    var errs []CrossError
    for _, r := range rules {
        if skipRules[r.Name] {
            continue
        }
        if r.Requires(input) {
            errs = append(errs, r.Check(input)...)
        }
    }
    return errs
}

// Rules returns the registered rule list (for status/reporting).
func Rules() []Rule {
    return rules
}
```

기존 `Run(input)` 시그니처 유지 — 호출부 변경 없음.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/crosscheck/types.go` | `Rule` struct 추가 |
| `internal/crosscheck/rules.go` | 신규 — 15개 Rule 등록 |
| `internal/crosscheck/crosscheck.go` | `Run()` → `RunRules()` 위임, `Rules()` export 추가. 기존 if-block 삭제, import 정리 |
| `internal/crosscheck/crosscheck_test.go` | 신규 — `RunRules` skipRules 동작 테스트 |

## 변경하지 않는 파일

기존 15개 Check 함수 파일(openapi_ddl.go, ssac_ddl.go, ...) — 시그니처·로직 변경 없음.

## 검증

1. `go test ./internal/crosscheck/...` — 기존 테스트 전량 통과
2. `go run ./cmd/fullend validate specs/gigbridge` — 기존과 동일한 출력
3. `crosscheck_test.go` — skipRules로 특정 rule 스킵 시 해당 에러 미출력 확인
4. `go vet ./...` — 정적 분석 통과

## 의존성

없음 — Phase025 이후 독립 실행 가능.
