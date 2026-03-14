# Phase027: Validate Rule 개선 — 중복 제거 + 신규 규칙 + mutest 정리

## 목표

Phase026 Rulebook 기반 위에서:
1. 중복 규칙 1건 제거
2. 신규 규칙 2건 추가
3. mutest 파일 방향 오류 2건 수정

## 의존성

Phase026 완료 후 실행.

---

## A. 중복 규칙 제거: States → OpenAPI

### 현황

`CheckStates` #4는 States transition event → OpenAPI operationId를 검증한다.

이 검증은 아래 두 규칙의 전이적 합성이다:
- `CheckStates` #1: States event → SSaC func 존재
- `CheckSSaCOpenAPI` Rule 3: SSaC func → OpenAPI operationId 존재

독자적으로 잡아내는 에러가 없으며, States event에 SSaC func이 없을 때 중복 에러(States→SSaC + States→OpenAPI)를 출력한다.

### 수정

`states.go` #4 블록(line 117~132) 삭제. 함수 시그니처에서 `doc *openapi3.T` 파라미터는 #5(States field → DDL)에서 사용하지 않으므로 제거 가능하나, 향후 확장 여지를 위해 유지한다.

`rules.go`에서 해당 Rule의 Name을 `"States ↔ SSaC/DDL"` 로 변경 (OpenAPI 제거).

---

## B. 신규 규칙 1: Func → SSaC 커버리지

### 동기

`CheckDDLCoverage`(DDL orphan table 검출)는 있지만, Func orphan 검출은 없다. 프로젝트 `func/` 디렉토리에 함수 스펙이 있지만 아무 SSaC `@call`도 참조하지 않으면 죽은 코드다.

### 구현

```go
// func_coverage.go
func CheckFuncCoverage(
    funcs []ssacparser.ServiceFunc,
    projectFuncSpecs []funcspec.FuncSpec,
) []CrossError {
    // SSaC @call에서 참조하는 pkg.Function 목록 수집
    referenced := make(map[string]bool) // "billing.HoldEscrow"
    for _, fn := range funcs {
        for _, seq := range fn.Sequences {
            if seq.Type == "call" && seq.Model != "" {
                referenced[seq.Model] = true
            }
        }
    }

    var errs []CrossError
    for _, spec := range projectFuncSpecs {
        key := spec.Package + "." + spec.Name
        if !referenced[key] {
            errs = append(errs, CrossError{
                Rule:       "Func → SSaC",
                Context:    key,
                Message:    fmt.Sprintf("func spec %q is not referenced by any SSaC @call", key),
                Level:      "WARNING",
                Suggestion: fmt.Sprintf("SSaC에서 @call %s를 추가하거나 func/%s를 제거하세요", key, spec.Package),
            })
        }
    }
    return errs
}
```

rules.go 등록:
```go
{
    Name: "Func → SSaC (coverage)", Source: "Func", Target: "SSaC",
    Requires: func(in *CrossValidateInput) bool {
        return in.ServiceFuncs != nil && len(in.ProjectFuncSpecs) > 0
    },
    Check: func(in *CrossValidateInput) []CrossError {
        return CheckFuncCoverage(in.ServiceFuncs, in.ProjectFuncSpecs)
    },
},
```

pkg/ 내장 함수는 프로젝트가 안 쓸 수 있으므로 `ProjectFuncSpecs`만 대상.

---

## C. 신규 규칙 2: STML → OpenAPI

### 동기

STML 페이지가 참조하는 API endpoint가 OpenAPI에 존재하는지 검증하는 crosscheck가 없다.

### 구현

STML 파서가 API call 정보를 추출하는지 확인 필요. `stmlparser.PageSpec`에 API endpoint 참조 정보가 있으면 구현 가능. 없으면 STML 파서 확장이 선행되어야 한다.

**선행 조건 확인 후 구현 여부 결정.** STML 파서에 API 참조 정보가 없으면 이 항목은 별도 Phase로 분리한다.

---

## D. mutest 파일 정리 (2건)

### 1. `config-policy.md` → `policy-config.md` 합산

MUT-CONFIG-POLICY-001(Claims→Rego)은 Policy가 Config를 소비하는 방향이다. `policy-config.md`에 합산하고 `config-policy.md` 삭제.

넘버링: MUT-CONFIG-POLICY-001 → MUT-POLICY-CONFIG-002

### 2. MUT-STATES-SSAC-002 → `ssac-states.md` 이동

MUT-STATES-SSAC-002(SSaC @state → States transition)는 SSaC가 States를 소비하는 방향이다. `ssac-states.md` 신규 생성 후 이동.

넘버링: MUT-STATES-SSAC-002 → MUT-SSAC-STATES-001

`config-ssac.md` → `ssac-config.md` rename (양쪽 빈 파일).

phase1.md, phase2.md 인덱스 테이블도 업데이트.

---

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/crosscheck/states.go` | #4 블록(States→OpenAPI) 삭제 |
| `internal/crosscheck/rules.go` | States rule Name 변경, CheckFuncCoverage rule 추가, (조건부) CheckSTMLOpenAPI rule 추가 |
| `internal/crosscheck/func_coverage.go` | 신규 |
| `internal/crosscheck/func_coverage_test.go` | 신규 |
| `files/mutests/policy-config.md` | MUT-POLICY-CONFIG-002 추가 |
| `files/mutests/config-policy.md` | 삭제 |
| `files/mutests/ssac-states.md` | 신규 — MUT-SSAC-STATES-001 |
| `files/mutests/states-ssac.md` | MUT-STATES-SSAC-002 제거 (001만 남음) |
| `files/mutests/ssac-config.md` | `config-ssac.md`에서 rename |
| `files/mutests/config-ssac.md` | 삭제 |
| `files/mutests/phase1.md` | 인덱스 업데이트 |
| `files/mutests/phase2.md` | 인덱스 업데이트 |
| `files/validates.md` | 현황 업데이트 |

## 검증

1. `go test ./internal/crosscheck/...` — 기존 + 신규 통과
2. `go run ./cmd/fullend validate specs/gigbridge` — States→OpenAPI 중복 에러 제거 확인
3. gigbridge에 orphan func 추가 → WARNING 검출 확인
4. `go vet ./...` 통과
