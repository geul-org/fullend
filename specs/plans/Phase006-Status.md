# ✅ 완료 — Phase 6 — status

## 목표
`fullend status <specs-dir>` 실행 시 SSOT 현황을 요약 출력한다.

## 선행 조건
Phase 2 완료 (detect, parser 호출 가능)

## 변경 파일

| 파일 | 작업 |
|---|---|
| `artifacts/internal/orchestrator/status.go` | 생성. 현황 집계 + 출력 |
| `artifacts/cmd/fullend/main.go` | 수정. status 서브커맨드 연결 |

## 집계 항목

| SSOT | 집계 방법 | 출력 |
|---|---|---|
| DDL | ssac SymbolTable.DDLTables 크기 + 컬럼 수 합산 | `12 tables, 47 columns` |
| OpenAPI | kin-openapi로 로드 → paths 순회 → operation 수 | `34 endpoints` |
| SSaC | ssac parser.ParseDir() → len(funcs) | `34 functions` |
| STML | stml parser.ParseDir() → len(pages) | `18 pages` |
| Terraform | `<specs>/terraform/*.tf` 파일 수 | `3 files` |

## 출력 포맷

```
SSOT Status:
  DDL        specs/db/                    12 tables, 47 columns
  OpenAPI    specs/api/openapi.yaml       34 endpoints
  SSaC       specs/backend/service/       34 functions
  STML       specs/frontend/              18 pages
  Terraform  specs/terraform/              3 files
```

존재하지 않는 SSOT는 출력하지 않는다.

## 검증 방법

- dummy-study 프로젝트로 실행 → 수치 일치 확인
- 빈 specs → 아무것도 출력 안 함
- 일부 SSOT만 존재 → 해당 항목만 출력
