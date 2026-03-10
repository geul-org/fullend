# Phase 014: crosscheck seq.Args → seq.Inputs 전환 + @call Input 타입 검증

## 배경

수정지시서008에서 CRUD(@get/@post/@put/@delete)도 `seq.Args` → `seq.Inputs` (map[string]string)으로 변경됨.
crosscheck 코드 중 `seq.Args`를 참조하는 곳이 아직 남아있어 빈 배열을 순회하게 됨.

또한 Phase 013에서 @call Inputs의 **필드명 존재 여부**만 검증했다.
`Amount: gig.Budget`처럼 필드명은 맞지만, func spec `Amount int` vs DDL `budget BIGINT` → `int64`로 타입이 불일치하면 컴파일 에러가 나는데 validate가 감지하지 못한다.

## 목표

1. crosscheck 전체에서 `seq.Args` 잔존 참조를 `seq.Inputs` 기반으로 전환
2. `Func ↔ SSaC` crosscheck Rule 2에 **Input value 타입 검증** 추가

## 타입 해석 로직

Input value의 형태에 따라 타입을 해석:

| value 패턴 | 타입 해석 |
|---|---|
| `request.Email` | OpenAPI request schema에서 필드 타입 조회 |
| `gig.Budget` | `definedVars`에서 source 타입(DDL 테이블명) → SymbolTable DDL 컬럼 타입 조회 |
| `currentUser.ID` | fullend.yaml claims 매핑 → 타입 (int64, string 등) |
| `"cancelled"` | 리터럴 → `string` |
| `config.*` | 스킵 (타입 불확실) |

기존 `resolveDDLColumnType()`, `resolveOpenAPIFieldType()`, `typesCompatible()` 함수가 이미 있으므로 재활용.

## 변경 파일

### 1. `internal/crosscheck/claims.go` — seq.Args 제거

`collectCurrentUserFields()`에서 `seq.Args` 순회(71행) 제거.
CRUD가 Inputs로 변경됐으므로 `seq.Inputs` 순회(77~83행)만 남기면 됨.

```go
// 삭제 대상 (71~75행)
for _, arg := range seq.Args {
    if arg.Source == "currentUser" && arg.Field != "" {
        result[arg.Field] = append(result[arg.Field], loc)
    }
}
```

### 2. `internal/crosscheck/ssac_ddl.go` — seq.Args → seq.Inputs 전환

`checkParamTypes()`에서 `seq.Args` 순회(99행) → `seq.Inputs` 기반으로 변경.
Input value에서 source가 `request`인 경우 key(필드명)를 DDL 컬럼과 대조.

```go
// 변경 전
for _, arg := range seq.Args {
    if arg.Source != "request" { continue }
    colName := pascalToSnake(arg.Field)
    ...
}

// 변경 후
for key, value := range seq.Inputs {
    parts := strings.SplitN(value, ".", 2)
    if parts[0] != "request" { continue }
    colName := pascalToSnake(key)
    ...
}
```

### 3. `internal/crosscheck/func.go` — @call Input 타입 검증

Rule 2 블록에 타입 검증 추가:

```go
// Rule 2: Input key names + types must match Request field names + types.
if inputCount > 0 {
    reqFieldMap := make(map[string]string) // name → type
    for _, rf := range spec.RequestFields {
        reqFieldMap[rf.Name] = rf.Type
    }
    for inputKey, inputValue := range seq.Inputs {
        reqType, exists := reqFieldMap[inputKey]
        if !exists {
            // 필드명 불일치 ERROR (기존)
            continue
        }
        // 타입 검증
        valueType := resolveInputValueType(inputValue, definedVars, symbolTable, openAPIDoc, sf.Name)
        if valueType != "" && !typesCompatible(valueType, reqType) {
            // ERROR: 타입 불일치
        }
    }
}
```

`resolveInputValueType()` 신규 함수:
- value를 `source.field`로 분리
- source에 따라 DDL/OpenAPI/리터럴 타입 해석

### 4. `internal/crosscheck/func_test.go`

- `TestCheckFuncs_InputTypeMatch` — 타입 일치 시 에러 없음
- `TestCheckFuncs_InputTypeMismatch` — `int` vs `int64` 불일치 감지

### 5. `internal/crosscheck/claims_test.go` (있다면)

- `seq.Args` → `seq.Inputs` 전환

### 6. `internal/crosscheck/ssac_ddl_test.go` (있다면)

- `seq.Args` → `seq.Inputs` 전환

## 의존성

- Phase 013 완료 (seq.Inputs 대응)
- `resolveDDLColumnType()`, `resolveOpenAPIFieldType()`, `typesCompatible()` 기존 함수

## 검증 방법

```bash
go test ./internal/crosscheck/ -v
go run ./cmd/fullend validate specs/dummy-gigbridge
```

- func spec `Amount int` vs DDL `budget int64` 불일치 시 ERROR 출력 확인
- func spec 수정 후 ERROR 해소 확인

## 상태: ✅ 완료
