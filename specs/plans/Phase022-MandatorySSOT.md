# Phase 022: SSOT 필수 검증 + --skip 옵션

## 목표

7개 SSOT를 기본 필수로 만들고, 없으면 ERROR를 출력한다. `--skip` 옵션으로 명시적으로 제외할 수 있게 한다.

```
현재:
  SSOT 파일 없음 → Skip (조용히 넘어감)

목표:
  SSOT 파일 없음 → ERROR (기본값)
  --skip states,policy → 해당 SSOT만 Skip
```

---

## 설계

### CLI 인터페이스

```bash
# 7개 전부 필수 (기본)
fullend validate specs/dummy-lesson

# states, terraform 제외
fullend validate --skip states,terraform specs/dummy-lesson

# gen도 동일
fullend gen --skip terraform specs/dummy-lesson artifacts/dummy-lesson

# status는 현황 출력이므로 --skip 불필요 (기존 유지)
fullend status specs/dummy-lesson
```

### --skip 허용 값

| 값 | SSOTKind | 디렉토리 |
|---|---|---|
| `openapi` | OpenAPI | `api/openapi.yaml` |
| `ddl` | DDL | `db/*.sql` |
| `ssac` | SSaC | `service/*.go` |
| `model` | Model | `model/*.go` |
| `stml` | STML | `frontend/*.html` |
| `states` | States | `states/*.md` |
| `policy` | Policy | `policy/*.rego` |
| `terraform` | Terraform | `terraform/*.tf` |

소문자로 통일. 잘못된 값이 들어오면 즉시 에러.

### 동작 변경

1. **detect**: 기존과 동일 (있는 것만 반환)
2. **validate**: detected에 없는 SSOT를 체크
   - `--skip`에 포함 → `Status: Skip, Summary: "skipped (--skip)"`
   - `--skip`에 미포함 → `Status: Fail, Summary: "required but not found"`
3. **cross-validate**: 필수 3종(OpenAPI, DDL, SSaC)이 모두 있어야 실행. 하나라도 없으면 skip (기존과 동일하나, 이제 그 전에 이미 ERROR가 뜨므로 실질적으로 도달 불가)
4. **gen**: validate 통과해야 gen 실행. skip된 SSOT의 gen 단계도 skip.

### 출력 예시

```
# 모든 SSOT 존재 시 (기존과 동일)
✓ OpenAPI      18 endpoints
✓ DDL          6 tables, 38 columns
✓ SSaC         18 service functions
✓ STML         7 pages, 12 bindings
✓ States       1 diagrams, 3 transitions
✓ Policy       1 files, 5 rules, 3 ownership mappings
✓ Terraform    2 files
✓ Cross        0 mismatches

# states, policy 없이 --skip 없이 실행 시
✓ OpenAPI      18 endpoints
✓ DDL          6 tables, 38 columns
✓ SSaC         18 service functions
✓ STML         7 pages, 12 bindings
✗ States       required but not found
✗ Policy       required but not found
✓ Terraform    2 files
- Cross        skipped (validation failed)

SSOT validation failed.

# --skip states,policy 시
✓ OpenAPI      18 endpoints
✓ DDL          6 tables, 38 columns
✓ SSaC         18 service functions
✓ STML         7 pages, 12 bindings
- States       skipped (--skip)
- Policy       skipped (--skip)
✓ Terraform    2 files
✓ Cross        0 mismatches

All SSOT sources are consistent.
```

---

## 구현

### 수정 파일

| 파일 | 변경 |
|---|---|
| `artifacts/cmd/fullend/main.go` | `--skip` 플래그 파싱, 쉼표 분리, 소문자 변환 |
| `artifacts/internal/orchestrator/validate.go` | `Validate(root, detected, skipKinds)` 시그니처 변경. 미감지+미스킵 SSOT에 Fail 출력 |
| `artifacts/internal/orchestrator/gen.go` | `Gen(root, detected, artifactsDir, skipKinds)` 시그니처 변경. skip 전달 |
| `artifacts/internal/orchestrator/detect.go` | `AllSSOTKinds()` 헬퍼 추가 (전체 Kind 목록 반환), `KindFromString(s)` 파싱 헬퍼 추가 |

### 새 파일

없음.

### 핵심 변경

#### detect.go — Kind 매핑

```go
// kindNames maps CLI --skip values to SSOTKind.
var kindNames = map[string]SSOTKind{
    "openapi":   KindOpenAPI,
    "ddl":       KindDDL,
    "ssac":      KindSSaC,
    "model":     KindModel,
    "stml":      KindSTML,
    "states":    KindStates,
    "policy":    KindPolicy,
    "terraform": KindTerraform,
}

func KindFromString(s string) (SSOTKind, bool) {
    k, ok := kindNames[strings.ToLower(s)]
    return k, ok
}

// AllSSOTKinds returns all SSOT kinds that fullend validates.
func AllSSOTKinds() []SSOTKind {
    return []SSOTKind{
        KindOpenAPI, KindDDL, KindSSaC, KindModel,
        KindSTML, KindStates, KindPolicy, KindTerraform,
    }
}
```

#### validate.go — 필수 검증

```go
func Validate(root string, detected []DetectedSSOT, skipKinds map[SSOTKind]bool) *reporter.Report {
    // ...
    for _, kind := range allKinds {
        d, ok := has[kind]
        if !ok {
            if skipKinds[kind] {
                // 명시적 skip
                report.Steps = append(report.Steps, reporter.StepResult{
                    Name:    string(kind),
                    Status:  reporter.Skip,
                    Summary: "skipped (--skip)",
                })
            } else {
                // 필수인데 없음 → ERROR
                report.Steps = append(report.Steps, reporter.StepResult{
                    Name:    string(kind),
                    Status:  reporter.Fail,
                    Summary: "required but not found",
                })
            }
            continue
        }
        // 기존 검증 로직...
    }
}
```

#### main.go — 플래그 파싱

```go
var skipFlag string
// validate 커맨드에 --skip 추가
flag.StringVar(&skipFlag, "skip", "", "comma-separated SSOT kinds to skip")

// 파싱
skipKinds := make(map[SSOTKind]bool)
if skipFlag != "" {
    for _, s := range strings.Split(skipFlag, ",") {
        kind, ok := orchestrator.KindFromString(strings.TrimSpace(s))
        if !ok {
            fmt.Fprintf(os.Stderr, "unknown SSOT kind: %q\n", s)
            os.Exit(1)
        }
        skipKinds[kind] = true
    }
}
```

---

## allKinds 업데이트

현재 `allKinds`에 Model, Terraform이 빠져 있다. 필수 검증을 위해 추가해야 한다.

```go
// 현재
var allKinds = []SSOTKind{KindOpenAPI, KindDDL, KindSSaC, KindSTML, KindStates, KindPolicy}

// 변경
var allKinds = []SSOTKind{KindOpenAPI, KindDDL, KindSSaC, KindModel, KindSTML, KindStates, KindPolicy, KindTerraform}
```

Model과 Terraform의 validate 케이스도 추가해야 한다:
- **Model**: `model/*.go` 파일이 존재하는지만 확인 (파싱은 SSaC가 담당). 파일 수 출력.
- **Terraform**: `terraform/*.tf` 파일이 존재하는지만 확인 (`terraform fmt` 검증은 gen 단계). 파일 수 출력.

---

## 문서 업데이트

### CLAUDE.md

CLI 명령어 섹션에 `--skip` 옵션 설명 추가:

```
### fullend validate [--skip kind,...] <specs-dir>
### fullend gen [--skip kind,...] <specs-dir> <artifacts-dir>
```

SSOT 원칙 섹션에 필수 정책 명시:

```
7개 SSOT는 기본 필수. 파일이 없으면 ERROR.
--skip으로 명시적 제외 가능 (openapi, ddl, ssac, model, stml, states, policy, terraform).
```

### README.md

Commands 섹션의 validate/gen 사용법에 `--skip` 옵션 추가.
출력 예시에 Model, Terraform 행 추가.

### manual-for-ai.md

fullend CLI 섹션에 `--skip` 옵션 설명 추가:

```
fullend validate [--skip kind,...] <specs-dir>
fullend gen [--skip kind,...] <specs-dir> <artifacts-dir>
```

기본 동작(7개 필수)과 `--skip` 예시 추가.

---

## 의존성

- 없음. 기존 코드 수정만으로 구현 가능.

## 검증

```bash
# 1. 기존 dummy-lesson (7개 SSOT 완비) — 변경 없이 통과
fullend validate specs/dummy-lesson
# ✓ 전부 통과

# 2. dummy-study (states, policy 추가 완료) — terraform 없으므로 에러
fullend validate specs/dummy-study
# ✗ Terraform    required but not found

# 3. --skip으로 terraform 제외
fullend validate --skip terraform specs/dummy-study
# ✓ 전부 통과

# 4. 잘못된 skip 값
fullend validate --skip foobar specs/dummy-lesson
# unknown SSOT kind: "foobar" (exit 1)

# 5. gen도 동일하게 동작
fullend gen --skip terraform specs/dummy-study artifacts/dummy-study
```
