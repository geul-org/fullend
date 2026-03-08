# ✅ 완료 — Phase 2 — validate 오케스트레이션

## 목표
`fullend validate <specs-dir>` 실행 시 specs 디렉토리 구조를 감지하고, 존재하는 SSOT에 대해 개별 검증을 순차 실행한다. 아직 cross-validate 없이 개별 검증만 통합.

## 선행 조건
Phase 1 완료

## 변경 파일

| 파일 | 작업 |
|---|---|
| `artifacts/internal/orchestrator/detect.go` | 생성. specs 디렉토리 스캔 → 어떤 SSOT가 존재하는지 감지 |
| `artifacts/internal/orchestrator/validate.go` | 생성. 검증 순서 제어 |
| `artifacts/cmd/fullend/main.go` | 수정. validate 서브커맨드 연결 |

## specs-dir은 프로젝트 루트

`fullend validate specs/dummy-study/` 처럼 프로젝트 루트를 지정한다.
ssac/stml CLI와 동일한 규칙.

## 프로젝트 디렉토리 감지 규칙

| SSOT | 존재 조건 | 경로 |
|---|---|---|
| OpenAPI | `<root>/api/openapi.yaml` 존재 | api/ |
| DDL | `<root>/db/*.sql` 존재 | db/, db/queries/ |
| SSaC | `<root>/service/*.go` 존재 | service/ |
| Model | `<root>/model/*.go` 존재 | model/ (ssac 외부 검증에 필요) |
| STML | `<root>/frontend/*.html` 존재 | frontend/, frontend/components/ |
| Terraform | `<root>/terraform/*.tf` 존재 | terraform/ |

존재하지 않는 SSOT는 건너뛴다 (에러 아님).

## 검증 실행 순서

```
1. OpenAPI — kin-openapi로 스키마 로드 + 구조 검증
2. DDL — ssac validator.LoadSymbolTable()로 DDL 파싱 (테이블/컬럼 추출)
3. SSaC — ssac parser.ParseDir() → validator.ValidateWithSymbols()
4. STML — stml parser.ParseDir() → validator.Validate()
```

각 단계의 결과를 []error로 수집한다. 한 단계가 실패해도 다음 단계를 계속 실행한다 (최대한 많은 에러를 한 번에 보여주기 위해).

## 의존성

| 패키지 | 호출 함수 |
|---|---|
| `github.com/getkin/kin-openapi/openapi3` | `openapi3.NewLoader().LoadFromFile()` |
| `github.com/geul-org/ssac/parser` | `ParseDir()` |
| `github.com/geul-org/ssac/validator` | `LoadSymbolTable()`, `ValidateWithSymbols()` |
| `github.com/geul-org/stml/parser` | `ParseDir()` |
| `github.com/geul-org/stml/validator` | `Validate()` |

## 검증 방법

- `fullend validate specs/dummy-study/` 실행
- 정상 프로젝트: 에러 0
- 일부 SSOT만 있는 경우: 해당 SSOT만 검증, 나머지 skip
- specs 디렉토리 없으면: 에러 메시지 + exit 1
