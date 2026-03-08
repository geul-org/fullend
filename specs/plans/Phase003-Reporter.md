# ✅ 완료 — Phase 3 — Reporter

## 목표
검증 결과를 ✓/✗ 포맷으로 통합 출력한다. exit code를 정의한다.

## 선행 조건
Phase 2 완료

## 변경 파일

| 파일 | 작업 |
|---|---|
| `artifacts/internal/reporter/reporter.go` | 생성. 결과 포매팅 + 출력 |
| `artifacts/internal/reporter/types.go` | 생성. StepResult 타입 정의 |
| `artifacts/internal/orchestrator/validate.go` | 수정. reporter 호출 |

## 출력 포맷

### 전체 통과
```
✓ OpenAPI      34 endpoints
✓ DDL          12 tables, 47 columns
✓ SSaC         34 service functions
✓ STML         18 pages, 43 bindings
✓ Cross        0 mismatches

All SSOT sources are consistent.
```

### 부분 실패
```
✓ OpenAPI      34 endpoints
✓ DDL          12 tables, 47 columns
✗ SSaC         2 errors
    reservation.go:CancelReservation @model Reservation.SoftDelete — method not found
    payment.go:RefundPayment @param Amount — not in request schema
✓ STML         18 pages, 43 bindings
— Cross        skipped (SSaC errors)

FAILED: Fix errors before codegen.
```

### 부분 존재 (skip)
```
✓ OpenAPI      34 endpoints
— DDL          not found, skipped
— SSaC         not found, skipped
✓ STML         18 pages, 43 bindings
— Cross        skipped (incomplete SSOT)

Partial validation passed.
```

## StepResult 타입

```go
type StepResult struct {
    Name    string           // "OpenAPI", "DDL", "SSaC", "STML", "Cross"
    Status  Status           // Pass, Fail, Skip
    Summary string           // "34 endpoints", "12 tables, 47 columns"
    Errors  []string         // 개별 에러 메시지
}

type Status int
const (
    Pass Status = iota
    Fail
    Skip
)
```

## exit code

| 상황 | exit code |
|---|---|
| 전체 통과 | 0 |
| 검증 실패 있음 | 1 |
| specs 디렉토리 없음 등 입력 에러 | 2 |

## 검증 방법

- 정상 프로젝트: 모든 ✓ + exit 0
- 에러 프로젝트: ✗ 항목에 에러 목록 + exit 1
- 빈 프로젝트: 모든 — (skip) + exit 0
