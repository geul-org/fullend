# Phase024: crosscheck 타입 검증 강화 + SSaC 리터럴 지원 + 문서 보완 ✅ 완료

## 목표

1. crosscheck `resolveInputValueType`의 타입 검증을 실질적으로 동작하게 수정한다 (MEDIUM)
2. SSaC 파서에 숫자/불리언/nil 리터럴 인식을 추가한다 (LOW)
3. func import 경로 제약을 매뉴얼에 명시한다 (LOW)

## 의존성

Phase022 (inflection) 완료 필수 — `modelToTable`이 `inflection.Plural`을 사용.

## 발견 경위

`files/dummy-zenflow-report2.md` 이슈 #2, #3, #4.

---

## 이슈 A: `resolveInputValueType`이 대부분 `""`을 반환하여 타입 검증 무력화 (MEDIUM)

**현상**: SSaC `@call worker.ProcessAction({Actions: actions})`에서 `actions`가 `[]model.Action`이지만, func spec의 `ProcessActionRequest.Actions`는 `[]worker.ActionItem`으로 선언됨. validate가 타입 불일치를 감지하지 못하고 빌드 시점까지 에러가 지연됨.

**근원 분석**: `resolveInputValueType` (:190-218)이 `""`을 반환하는 경로가 **4개**:

1. **bare variable** (:197-198) — `len(parts) < 2` → `""`. 항상 skip.
2. **currentUser** (:208-209) — 의도적 skip (합리적).
3. **definedVars miss** (:213-215) — 변수 미발견 → `""`.
4. **resolveDDLColumnType 실패** (:217, :222-245) — **가장 심각한 근원 버그**:
   - `definedVars[var]`는 PascalCase 모델명 (`"Workflow"`)
   - `resolveDDLColumnType`이 `DDLTables["Workflow"]`로 조회 → **실패** (키는 `"workflows"`)
   - `strings.ToLower` fallback이 `"workflow"`로 시도 → **역시 실패** (복수형 필요)

추가로, `@call` 결과 변수의 타입은 func Response 타입명(`"CheckCreditsResponse"`)이지 DDL 테이블명이 아니므로 `resolveDDLColumnType`으로는 **원천적으로 resolve 불가**.

**기존 자산**: `crosscheck/ssac_ddl.go:142`에 `modelToTable("Workflow")` → `"workflows"` 변환 함수가 **이미 존재** (Phase022에서 inflection으로 교체됨).

**수정 방안**: `resolveInputValueType` 3단계 강화

```go
// crosscheck/func.go - resolveInputValueType 수정

func resolveInputValueType(value string, definedVars map[string]string,
    st *ssacvalidator.SymbolTable, doc *openapi3.T, funcName string,
    funcSpecs []funcspec.FuncSpec) string {  // funcSpecs 파라미터 추가

    // 1) Literal string
    if strings.HasPrefix(value, "\"") { return "string" }

    // 2) Numeric/boolean literal (이슈 B와 연동)
    if ssacparser.IsLiteral(value) { return inferLiteralType(value) }

    // 3) Bare variable (dot 없음) — 변수 자체의 타입 반환
    parts := strings.SplitN(value, ".", 2)
    if len(parts) < 2 {
        typeName, ok := definedVars[value]
        if !ok { return "" }
        return typeName
    }

    source, field := parts[0], parts[1]
    if source == "request" { return resolveOpenAPIFieldType(doc, funcName, field) }
    if source == "currentUser" { return "" }

    typeName, ok := definedVars[source]
    if !ok { return "" }

    // 4) DDL 모델 필드: modelToTable로 변환 후 조회
    tableName := modelToTable(typeName)  // "Workflow" → "workflows" (ssac_ddl.go:142, 같은 패키지)
    if goType := resolveDDLColumnType(st, tableName, field); goType != "" {
        return goType
    }

    // 5) Func Response 필드: funcSpecs에서 조회
    return resolveFuncResponseFieldType(funcSpecs, typeName, field)
}

// 신규: func Response 필드 타입 조회
// 제한: SSaC result type이 "<FuncName>Response" 규약을 따를 때만 동작.
func resolveFuncResponseFieldType(specs []funcspec.FuncSpec, respTypeName, field string) string {
    for _, spec := range specs {
        if spec.Name+"Response" == respTypeName {
            for _, f := range spec.ResponseFields {
                if f.Name == field { return f.Type }
            }
        }
    }
    return ""
}
```

**`resolveDDLColumnType` 칼럼명 변환 보강** (:222-245):

SSaC 필드 참조는 PascalCase (`wf.OrgID` → field=`"OrgID"`), DDLTable.Columns 키는 snake_case (`"org_id"`). `toSnakeCase` fallback 추가:

```go
func resolveDDLColumnType(st *ssacvalidator.SymbolTable, tableName, columnName string) string {
    if st == nil || st.DDLTables == nil { return "" }
    table, ok := st.DDLTables[tableName]
    if !ok { return "" }  // modelToTable로 이미 변환되었으므로 lowercase fallback 불필요
    // 1) Exact match
    if goType, ok := table.Columns[columnName]; ok { return goType }
    // 2) PascalCase→snake_case 변환: "OrgID" → "org_id"
    snakeCol := toSnakeCase(columnName)
    if goType, ok := table.Columns[snakeCol]; ok { return goType }
    // 3) Case-insensitive fallback (기존 유지)
    for colName, goType := range table.Columns {
        if strings.EqualFold(colName, columnName) { return goType }
    }
    return ""
}
```

**참고**: crosscheck 패키지에 `toSnakeCase` 함수가 이미 존재 (`func.go:393`).

**구현 흐름**:
1. `resolveDDLColumnType(:222)` — `modelToTable` 결과의 `tableName`으로 조회 + `toSnakeCase(columnName)` 변환 추가
2. DDL miss 시 `resolveFuncResponseFieldType` fallback
3. bare variable 경로 — dot 없는 변수도 `definedVars`에서 타입명 반환
4. `resolveInputValueType` 시그니처에 `funcSpecs []funcspec.FuncSpec` 추가
5. `CheckFuncs(:133)` 호출부에서 `allSpecs := append(fullendPkgSpecs, projectFuncSpecs...)` 조합 후 전달

---

## 이슈 B: SSaC에서 리터럴 값 지원 부재 — 숫자, 불리언, nil (LOW)

**현상**: `@post ... CreditsSpent: 1` 작성 시, `1`이 변수명으로 해석되어 crosscheck에서 "arg source "1" 미정의" WARNING 발생.

**근원 분석**: SSaC 리터럴 시스템이 **문자열 리터럴(`"quoted"`)만** 지원:

| 리터럴 | 예시 | parseArg 결과 | crosscheck 결과 |
|---|---|---|---|
| 숫자 | `1`, `42`, `3.14` | `Arg{Source: "1"}` (bare variable) | "1" 미정의 WARNING |
| 불리언 | `true`, `false` | `Arg{Source: "true"}` (bare variable) | "true" 미정의 WARNING |
| nil | `nil` | `Arg{Source: "nil"}` (bare variable) | "nil" 미정의 WARNING |

**수정 방안**: 통합 `IsLiteral` 함수 도입 — parser 패키지에 export, crosscheck에서 `ssacparser.IsLiteral()` 호출.

```go
// internal/ssac/parser/parser.go — 신규 export 헬퍼

// IsLiteral checks if a string is a Go literal value (not a variable reference).
func IsLiteral(s string) bool {
    if s == "true" || s == "false" || s == "nil" {
        return true
    }
    if len(s) == 0 {
        return false
    }
    start := 0
    if s[0] == '-' { start = 1 }
    if start >= len(s) { return false }
    dotSeen := false
    for i := start; i < len(s); i++ {
        if s[i] == '.' && !dotSeen { dotSeen = true; continue }
        if s[i] < '0' || s[i] > '9' { return false }
    }
    return true
}
```

**parseArg 수정** (:530-544):
```go
func parseArg(s string) Arg {
    s = strings.TrimSpace(s)
    if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
        return Arg{Literal: s[1 : len(s)-1]}
    }
    // numeric, boolean, nil literal — dot 검사보다 먼저 (3.14가 source.Field로 파싱되지 않도록)
    if IsLiteral(s) {
        return Arg{Literal: s}
    }
    dotIdx := strings.IndexByte(s, '.')
    if dotIdx > 0 {
        return Arg{Source: s[:dotIdx], Field: s[dotIdx+1:]}
    }
    return Arg{Source: s}
}
```

**crosscheck Rule 4 수정** (:162-181):
```go
for _, value := range seq.Inputs {
    if strings.HasPrefix(value, "\"") { continue }
    if ssacparser.IsLiteral(value) { continue }  // 추가
    // ... 기존 로직
}
```

**resolveInputValueType 연동** (이슈 A와 연결):
이슈 A에서 `resolveInputValueType`에 리터럴 타입 추론 추가:
```go
// crosscheck/func.go — 신규 로컬 헬퍼
func inferLiteralType(s string) string {
    if s == "true" || s == "false" { return "bool" }
    if s == "nil" { return "" }
    if strings.Contains(s, ".") { return "float64" }
    return "int"
}
```

---

## 이슈 C: func import 경로 제약 — 매뉴얼에 미명시 (LOW)

**현상**: SSaC에서 `import "github.com/org/project/func/billing"`으로 작성 시 에러. `internal/` 또는 `pkg/` 하위여야 함.

**수정 방안**: 매뉴얼 보완 (코드 변경 없음)
- `artifacts/manual-for-ai.md` Func Spec 섹션에 추가: "SSaC import 경로는 `internal/<pkg>` 형식으로 작성. func 스펙은 `specs/<project>/func/<pkg>/`에 위치하지만, 코드 생성 시 `artifacts/<project>/backend/internal/<pkg>/`로 복사됨"
- `artifacts/AGENTS.md` Step 2 SSaC 행 Notes에 동일 내용 추가
- `artifacts/manual-for-ai.md` Args Format 섹션에 리터럴 지원 명시

---

## 변경 파일

### 이슈 A (crosscheck 강화)
- `internal/crosscheck/func.go` (:190-218 resolveInputValueType 3단계 강화, :133 funcSpecs 전달, :222-245 resolveDDLColumnType에 toSnakeCase 변환 추가, resolveFuncResponseFieldType 신규, inferLiteralType 신규)
- `internal/crosscheck/func_test.go` (CheckFuncs 호출부 14곳 — resolveInputValueType 시그니처 변경은 내부 함수이므로 테스트 시그니처는 불변. 신규 테스트 케이스 추가: DDL 필드 타입 resolve, func Response 필드 타입 resolve, bare variable 타입 비교)

### 이슈 B (리터럴 지원)
- `internal/ssac/parser/parser.go` (:530-544 parseArg 수정, `IsLiteral` export 함수 신규 추가)

### 이슈 C (문서)
- `artifacts/manual-for-ai.md` (Func Spec 섹션 + Args Format 섹션)
- `artifacts/AGENTS.md` (Step 2 SSaC 행 Notes)

## 검증 방법

1. `@call billing.CheckCredits({OrgID: wf.OrgID})` — DDL 모델 필드 `wf.OrgID` → `resolveInputValueType`이 `"int64"` 반환 확인 (`modelToTable` 경유)
2. `@call worker.ProcessAction({WorkflowID: wf.ID})` — DDL 모델 필드 `wf.ID` → `"int64"` 반환 확인
3. `cr.Balance` (func Response 필드) — `resolveFuncResponseFieldType` 경유 → `"int64"` 반환 확인
4. bare variable `actions` (타입 `[]Action`) → `definedVars`에서 `"[]Action"` 반환 → `typesCompatible` 비교 동작 확인
5. `CreditsSpent: 1` → `fullend validate` WARNING 없음 + `fullend gen` → `go build` 성공 확인
6. `IsActive: true` → WARNING 없음 확인
7. `parseArg("42")` → `Arg{Literal: "42"}` 확인 (Source 비어있음)
8. 매뉴얼 수정 확인 — func import 경로 설명 존재 여부
9. `go test ./internal/crosscheck/...` 기존 테스트 통과 + 신규 테스트 케이스 통과
10. `go test ./internal/ssac/parser/...` 기존 테스트 통과

## whyso 이력

| 파일 | 이력 |
|---|---|
| `internal/crosscheck/func.go` | 2026-03-10 생성, 수정 이력 없음 |
| `internal/crosscheck/func_test.go` | 2026-03-10 생성, 수정 이력 없음 |
| `internal/ssac/parser/parser.go` | 없음 (최초 생성 후 미수정) |
