# Phase023: `@empty`/`@exists` 가드의 FieldTypeResolver 도입 ✅ 완료

## 목표

SSaC `@empty cr.Balance`에서 `Balance`가 `int64`일 때 `== nil` 대신 `== 0`을 생성하도록 codegen을 수정한다.

## 의존성

Phase022 (inflection) 완료 필수 — `inflection.Plural(toSnakeCase(modelName))`으로 모델→테이블 변환.

## 발견 경위

`files/dummy-zenflow-report2.md` 이슈 #1 (HIGH) — 컴파일 에러 직결.

## 현상

SSaC `@empty cr.Balance "msg" 402`에서 `Balance`가 `int64`일 때, 코드젠이 `cr.Balance == nil` 비교를 생성하여 컴파일 에러 발생.

## 원인

`internal/ssac/generator/go_helpers.go:119-122` — `resultTypes` 맵이 변수 레벨만 추적 (`"cr"` → `"CheckCreditsResponse"`). dotted field나 DDL 모델 필드의 Go 타입을 알 수 없어 `zeroValueChecks`가 항상 `default` → `== nil`로 처리.

## 근원 분석

codegen의 `resultTypes map[string]string`은 `변수명 → 타입명`의 1레벨 매핑이다. 하지만 `@empty`/`@exists`는 3가지 타겟 패턴이 존재:
1. `@empty wf` — DDL 모델 변수 (pointer → `== nil` 정상)
2. `@empty cr.Balance` — func Response struct의 필드 (int64 → `== 0` 필요)
3. `@empty wf.OrgID` — DDL 모델의 필드 (int64 → `== 0` 필요)

기존 시스템에 **이미 모든 타입 정보가 존재**:
- DDL 필드 타입: `SymbolTable.DDLTables["workflows"].Columns["org_id"]` → `"int64"` (snake_case)
- Func Response 필드 타입: `funcspec.FuncSpec.ResponseFields[i]` → `{Name: "Balance", Type: "int64"}`
- 단, 이 정보가 codegen의 `buildTemplateData`까지 전달되지 않는 것이 문제

## 수정 방안

### 1. GoTarget에 FuncSpecs 필드 추가

현재 `GoTarget`은 빈 struct이고 `GenerateFunc(sf, st)` 시그니처에 funcSpecs가 없음:

```go
// go_target.go:18 수정
type GoTarget struct {
    FuncSpecs []funcspec.FuncSpec
}

// target.go:11 — Target 인터페이스는 변경 불필요 (GoTarget 내부 필드이므로)
// generator.go:14 — Generate() 시그니처에 funcSpecs 추가
func Generate(funcs []parser.ServiceFunc, outDir string, st *validator.SymbolTable, funcSpecs []funcspec.FuncSpec) error {
    return GenerateWith(&GoTarget{FuncSpecs: funcSpecs}, funcs, outDir, st)
}
// generator.go:19 — GenerateFunc() 시그니처에 funcSpecs 추가
func GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable, funcSpecs []funcspec.FuncSpec) ([]byte, error) {
    return (&GoTarget{FuncSpecs: funcSpecs}).GenerateFunc(sf, st)
}
```

### 2. FieldTypeResolver 신규 파일

```go
// internal/ssac/generator/field_resolver.go (신규)
type varSource struct {
    Kind      string // "ddl" or "func"
    ModelName string // DDL: "Workflow", Func: "CheckCredits"
}

type FieldTypeResolver struct {
    vars map[string]varSource
    st   *validator.SymbolTable
    fs   []funcspec.FuncSpec
}

// ResolveFieldType("cr.Balance") → "int64"
// ResolveFieldType("wf.OrgID")   → "int64"
// ResolveFieldType("wf")         → "" (변수 자체는 pointer → default nil 유지)
func (r *FieldTypeResolver) ResolveFieldType(target string) string {
    parts := strings.SplitN(target, ".", 2)
    if len(parts) < 2 {
        return "" // 변수 자체 → default nil 유지
    }
    varName, fieldName := parts[0], parts[1]
    src, ok := r.vars[varName]
    if !ok {
        return ""
    }
    switch src.Kind {
    case "ddl":
        // Phase022에서 도입한 inflection.Plural 사용
        tableName := inflection.Plural(toSnakeCase(src.ModelName))
        if table, ok := r.st.DDLTables[tableName]; ok {
            // 필드명 PascalCase→snake_case 변환: "OrgID" → "org_id"
            snakeField := toSnakeCase(fieldName)
            if goType, ok := table.Columns[snakeField]; ok {
                return goType
            }
        }
    case "func":
        // "CheckCredits" → funcspec 탐색 → ResponseFields[i].Type
        for _, spec := range r.fs {
            if spec.Name == src.ModelName {
                for _, f := range spec.ResponseFields {
                    if f.Name == fieldName {
                        return f.Type
                    }
                }
            }
        }
    }
    return ""
}
```

### 3. 구현 흐름

1. `go_handler.go:71-76`: `resultTypes` 구성 시 동시에 `FieldTypeResolver.vars`도 구축
   - `@get`/`@post` 결과 → `varSource{Kind: "ddl", ModelName: seq.Result.Type}` 등록
   - `@call` 결과 → `varSource{Kind: "func", ModelName: strings.SplitN(seq.Model, ".", 2)[1]}` 등록 (예: `seq.Model="billing.CheckCredits"` → `ModelName="CheckCredits"`)
   - `FieldTypeResolver{vars: vars, st: st, fs: g.FuncSpecs}` 생성 (`g`는 GoTarget receiver)
2. `buildTemplateData` (:64) 파라미터에 `resolver *FieldTypeResolver` 추가
3. `go_helpers.go:119-122`: dotted target 시 `resolver.ResolveFieldType(seq.Target)` 호출 → `zeroValueChecks`에 정확한 타입 전달
4. `go_handler.go:94` (HTTP) 및 `:182` (Subscribe) — `buildTemplateData` 호출부에 `resolver` 인자 추가

### 4. orchestrator 연결

orchestrator는 `Generate()`가 아닌 `GenerateWith(profile.Backend, ...)` (:299)를 호출. `profile.Backend`는 `DefaultProfile()` → `&GoTarget{}` (:17). funcSpecs 주입:
```go
// orchestrator/gen.go:genSSaC() — GenerateWith 호출 전 (line 298 부근)
if gt, ok := profile.Backend.(*ssacgenerator.GoTarget); ok {
    gt.FuncSpecs = append(parsed.PkgFuncSpecs, parsed.FuncSpecs...)
}
```

## 주의사항

- **DDL 칼럼명 변환**: SSaC 필드 참조는 PascalCase (`wf.OrgID`), DDLTable.Columns 키는 snake_case (`org_id`). `toSnakeCase("OrgID")` → `"org_id"` 변환이 `ResolveFieldType` 내부에 포함됨. generator 패키지에 `toSnakeCase` 함수가 이미 존재 (`generator.go:113`).
- **모델명→테이블명 변환**: `sqlFileToModel`은 `filename→model` 정방향 변환이므로 역변환에 사용 불가. `inflection.Plural(toSnakeCase(modelName))` 사용 (Phase022에서 정규화된 패턴).
- **영향 범위**: `@empty`와 `@exists` 모두 같은 `zeroValueChecks`를 사용하므로 동시 수정됨.

## 변경 파일

- `internal/ssac/generator/field_resolver.go` (신규: varSource, FieldTypeResolver struct + ResolveFieldType)
- `internal/ssac/generator/go_target.go` (:18 GoTarget struct에 FuncSpecs 필드 추가)
- `internal/ssac/generator/generator.go` (:14 Generate, :19 GenerateFunc 시그니처에 funcSpecs 추가)
- `internal/ssac/generator/go_handler.go` (:71-76, :162-167 FieldTypeResolver 구축, :94, :182 buildTemplateData 호출부에 resolver 전달)
- `internal/ssac/generator/go_helpers.go` (:64 buildTemplateData 시그니처에 resolver 추가, :119-122 resolver 사용)
- `internal/ssac/generator/generator_test.go` (:14 GenerateFunc 시그니처 변경에 따른 호출부 수정)
- `internal/orchestrator/gen.go` (:299 genSSaC — `GenerateWith` 호출 전 GoTarget에 FuncSpecs 주입)

## 검증 방법

1. `@empty cr.Balance "msg" 402` (func Response int64 필드) → `go build` 성공, 생성 코드에 `== 0` 확인
2. `@empty wf.OrgID "msg"` (DDL 모델 int64 필드) → `go build` 성공, 생성 코드에 `== 0` 확인
3. `@empty wf "msg"` (DDL 모델 pointer) → 기존과 동일하게 `== nil` 확인
4. `go test ./internal/ssac/generator/...` 기존 테스트 통과

## whyso 이력

| 파일 | 이력 |
|---|---|
| `internal/ssac/generator/go_helpers.go` | 없음 (최초 생성 후 미수정) |
| `internal/ssac/generator/go_handler.go` | 없음 (최초 생성 후 미수정) |
| `internal/ssac/generator/go_target.go` | 없음 (최초 생성 후 미수정) |
| `internal/ssac/generator/generator.go` | 없음 (최초 생성 후 미수정) |
| `internal/ssac/generator/generator_test.go` | 없음 (최초 생성 후 미수정) |
| `internal/orchestrator/gen.go` | 2026-03-10 생성, 수정 이력 없음 |
